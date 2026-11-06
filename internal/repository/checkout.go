package repository

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gitlite/internal/index"
	"gitlite/internal/object"
)

var (
	// ErrDirtyWorkingTree is returned when checking out would overwrite untracked or uncommitted changes.
	ErrDirtyWorkingTree = errors.New("your local changes would be overwritten by checkout")
)

// FlattenTree walks an object.Tree recursively and returns a map of relative path -> TreeEntry.
func FlattenTree(store *object.Store, rootTreeHash object.Hash) (map[string]object.TreeEntry, error) {
	files := make(map[string]object.TreeEntry)

	var walk func(treeHash object.Hash, currentPrefix string) error
	walk = func(treeHash object.Hash, currentPrefix string) error {
		tree, err := store.ReadTree(treeHash)
		if err != nil {
			return err
		}

		for _, entry := range tree.Entries {
			entryPath := entry.Name
			if currentPrefix != "" {
				entryPath = currentPrefix + "/" + entry.Name
			}

			if entry.Mode == object.ModeDirectory {
				if err := walk(entry.Hash, entryPath); err != nil {
					return err
				}
			} else {
				files[entryPath] = entry
			}
		}
		return nil
	}

	if err := walk(rootTreeHash, ""); err != nil {
		return nil, fmt.Errorf("flattening tree %s: %w", rootTreeHash, err)
	}

	return files, nil
}

// Checkout checks out a branch name or commit hash safely into the working tree.
func (r *Repository) Checkout(target string) error {
	refMgr := r.Refs()

	// 1. Resolve target to commit hash
	var targetCommitHash object.Hash
	isBranch := false

	branchHash, err := refMgr.ResolveBranch(target)
	if err == nil {
		targetCommitHash = branchHash
		isBranch = true
	} else {
		// Try parsing as commit hash
		h := object.Hash(target)
		if err := h.Validate(); err != nil {
			return fmt.Errorf("pathspec '%s' did not match any file(s) known to gitlite", target)
		}
		targetCommitHash = h
	}

	// 2. Read target commit & root tree
	targetCommit, err := r.Store.ReadCommit(targetCommitHash)
	if err != nil {
		return fmt.Errorf("reading target commit %s: %w", targetCommitHash, err)
	}

	targetFiles, err := FlattenTree(r.Store, targetCommit.TreeHash)
	if err != nil {
		return err
	}

	// 3. Safety Check: Verify working tree against index
	// Prevent clobbering uncommitted modifications or untracked files
	currIdx, err := r.LoadIndex()
	if err != nil {
		return err
	}

	for relPath := range targetFiles {
		fullPath := filepath.Join(r.WorkTree, filepath.FromSlash(relPath))
		if info, err := os.Stat(fullPath); err == nil && !info.IsDir() {
			entry, staged := currIdx.Get(relPath)
			if !staged {
				// Untracked file in working tree would be overwritten!
				return fmt.Errorf("%w: untracked file %s", ErrDirtyWorkingTree, relPath)
			}
			// Check if file has local uncommitted modifications
			if info.Size() != entry.Size || info.ModTime().UnixNano() != entry.ModTimeNano {
				data, _ := os.ReadFile(fullPath)
				blob := object.NewBlob(data)
				canonical := object.EncodeCanonical(blob)
				if object.ComputeHash(canonical) != entry.Hash {
					return fmt.Errorf("%w: local changes in %s", ErrDirtyWorkingTree, relPath)
				}
			}
		}
	}

	// 4. Clean up files tracked in current index but NOT in target commit
	for relPath := range currIdx.Entries {
		if _, existsInTarget := targetFiles[relPath]; !existsInTarget {
			fullPath := filepath.Join(r.WorkTree, filepath.FromSlash(relPath))
			_ = os.Remove(fullPath)
			// Clean up parent directory if empty
			parent := filepath.Dir(fullPath)
			for parent != r.WorkTree && parent != "." {
				if err := os.Remove(parent); err != nil {
					break // Directory not empty
				}
				parent = filepath.Dir(parent)
			}
		}
	}

	// 5. Restore files from target commit to working tree and populate new index
	newIdx := index.New()

	for relPath, entry := range targetFiles {
		fullPath := filepath.Join(r.WorkTree, filepath.FromSlash(relPath))
		parentDir := filepath.Dir(fullPath)
		if err := os.MkdirAll(parentDir, 0755); err != nil {
			return fmt.Errorf("creating directory %s: %w", parentDir, err)
		}

		blob, err := r.Store.ReadBlob(entry.Hash)
		if err != nil {
			return fmt.Errorf("reading blob %s for %s: %w", entry.Hash, relPath, err)
		}

		perm := os.FileMode(0644)
		if entry.Mode == object.ModeExecutable {
			perm = 0755
		}

		if err := os.WriteFile(fullPath, blob.Data, perm); err != nil {
			return fmt.Errorf("writing file %s: %w", fullPath, err)
		}

		info, err := os.Stat(fullPath)
		if err != nil {
			return fmt.Errorf("stating written file %s: %w", fullPath, err)
		}

		newIdx.Add(index.Entry{
			Path:        relPath,
			Hash:        entry.Hash,
			Mode:        entry.Mode,
			Size:        info.Size(),
			ModTimeNano: info.ModTime().UnixNano(),
		})
	}

	// 6. Save new index
	if err := r.SaveIndex(newIdx); err != nil {
		return fmt.Errorf("saving checkout index: %w", err)
	}

	// 7. Update HEAD pointer
	if isBranch {
		if err := refMgr.SetHeadBranch(target); err != nil {
			return fmt.Errorf("updating HEAD to branch %s: %w", target, err)
		}
	} else {
		if err := refMgr.SetHeadDetached(targetCommitHash); err != nil {
			return fmt.Errorf("updating HEAD to detached commit %s: %w", targetCommitHash, err)
		}
	}

	return nil
}
