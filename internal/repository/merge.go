package repository

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"gitlite/internal/index"
	"gitlite/internal/merge"
	"gitlite/internal/object"
)

var (
	// ErrMergeConflict is returned when merge encounters unresolved conflicts.
	ErrMergeConflict = errors.New("automatic merge failed; fix conflicts and then commit the result")

	// ErrAlreadyUpToDate is returned when target commit is already an ancestor of HEAD.
	ErrAlreadyUpToDate = errors.New("already up to date")
)

// MergeResult encapsulates the outcome of a branch merge.
type MergeResult struct {
	FastForward   bool
	MergeCommit   object.Hash
	Conflicts     []string
	MergedFiles   []string
}

// Merge merges targetBranchOrCommit into the currently checked-out branch.
func (r *Repository) Merge(target string) (*MergeResult, error) {
	refMgr := r.Refs()

	// 1. Resolve current HEAD
	headHash, currentBranch, err := refMgr.ResolveHead()
	if err != nil {
		return nil, fmt.Errorf("resolving HEAD: %w", err)
	}

	// 2. Resolve incoming target commit
	var incomingHash object.Hash
	branchHash, err := refMgr.ResolveBranch(target)
	if err == nil {
		incomingHash = branchHash
	} else {
		h := object.Hash(target)
		if err := h.Validate(); err != nil {
			return nil, fmt.Errorf("target '%s' is neither a valid branch nor commit hash", target)
		}
		incomingHash = h
	}

	if headHash == incomingHash {
		return nil, ErrAlreadyUpToDate
	}

	// 3. Find Lowest Common Ancestor
	lca, err := merge.FindCommonAncestor(r.Store, headHash, incomingHash)
	if err != nil {
		return nil, fmt.Errorf("finding common ancestor: %w", err)
	}

	// 4. Fast-forward check: If LCA == HEAD, fast-forward directly to incoming commit
	if lca == headHash {
		if err := r.Checkout(string(incomingHash)); err != nil {
			return nil, fmt.Errorf("fast-forward checkout: %w", err)
		}
		// If on a named branch, advance branch ref
		if currentBranch != "" {
			if err := refMgr.SetHeadBranch(currentBranch); err != nil {
				return nil, err
			}
			if err := refMgr.UpdateHead(incomingHash); err != nil {
				return nil, err
			}
		}
		return &MergeResult{
			FastForward: true,
			MergeCommit: incomingHash,
		}, nil
	}

	// If LCA == incoming, already up to date!
	if lca == incomingHash {
		return nil, ErrAlreadyUpToDate
	}

	// 5. True 3-Way Merge
	// Flatten ancestor, current, and incoming trees
	var baseFiles map[string]object.TreeEntry
	if lca != "" {
		lcaCommit, err := r.Store.ReadCommit(lca)
		if err != nil {
			return nil, fmt.Errorf("reading ancestor commit %s: %w", lca, err)
		}
		baseFiles, err = FlattenTree(r.Store, lcaCommit.TreeHash)
		if err != nil {
			return nil, err
		}
	} else {
		baseFiles = make(map[string]object.TreeEntry)
	}

	headCommit, err := r.Store.ReadCommit(headHash)
	if err != nil {
		return nil, err
	}
	headFiles, err := FlattenTree(r.Store, headCommit.TreeHash)
	if err != nil {
		return nil, err
	}

	incomingCommit, err := r.Store.ReadCommit(incomingHash)
	if err != nil {
		return nil, err
	}
	incFiles, err := FlattenTree(r.Store, incomingCommit.TreeHash)
	if err != nil {
		return nil, err
	}

	// Collect union of all paths
	allPathsMap := make(map[string]bool)
	for p := range baseFiles {
		allPathsMap[p] = true
	}
	for p := range headFiles {
		allPathsMap[p] = true
	}
	for p := range incFiles {
		allPathsMap[p] = true
	}

	var allPaths []string
	for p := range allPathsMap {
		allPaths = append(allPaths, p)
	}
	sort.Strings(allPaths)

	var conflicts []string
	var mergedFiles []string
	idx := index.New()

	for _, path := range allPaths {
		baseEntry, inBase := baseFiles[path]
		headEntry, inHead := headFiles[path]
		incEntry, inInc := incFiles[path]

		var baseText, headText, incText string

		if inBase {
			b, err := r.Store.ReadBlob(baseEntry.Hash)
			if err != nil {
				return nil, fmt.Errorf("reading base blob %s for %s: %w", baseEntry.Hash, path, err)
			}
			baseText = string(b.Data)
		}
		if inHead {
			b, err := r.Store.ReadBlob(headEntry.Hash)
			if err != nil {
				return nil, fmt.Errorf("reading head blob %s for %s: %w", headEntry.Hash, path, err)
			}
			headText = string(b.Data)
		}
		if inInc {
			b, err := r.Store.ReadBlob(incEntry.Hash)
			if err != nil {
				return nil, fmt.Errorf("reading incoming blob %s for %s: %w", incEntry.Hash, path, err)
			}
			incText = string(b.Data)
		}

		fullPath := filepath.Join(r.WorkTree, filepath.FromSlash(path))

		// Case 1: Deleted on one side, unchanged on the other
		if !inHead && inInc && inBase && incEntry.Hash == baseEntry.Hash {
			// Deleted in HEAD, untouched in incoming -> stay deleted
			_ = os.Remove(fullPath)
			continue
		}
		if !inInc && inHead && inBase && headEntry.Hash == baseEntry.Hash {
			// Deleted in incoming, untouched in HEAD -> delete in working tree
			_ = os.Remove(fullPath)
			continue
		}

		// Case 2: Three-way content resolution
		res := merge.ThreeWayMergeContent(baseText, headText, incText)

		// Write merged/conflicted content to working tree
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(fullPath, []byte(res.Content), 0644); err != nil {
			return nil, err
		}

		// Stage blob into index
		mergedBlob := object.NewBlob([]byte(res.Content))
		h, err := r.Store.Write(mergedBlob)
		if err != nil {
			return nil, err
		}

		info, _ := os.Stat(fullPath)
		idx.Add(index.Entry{
			Path:        path,
			Hash:        h,
			Mode:        object.ModeRegular,
			Size:        int64(len(res.Content)),
			ModTimeNano: info.ModTime().UnixNano(),
		})

		if res.Conflict {
			conflicts = append(conflicts, path)
		} else {
			mergedFiles = append(mergedFiles, path)
		}
	}

	// Save merged index to disk
	if err := r.SaveIndex(idx); err != nil {
		return nil, err
	}

	// If there were conflicts, halt and require manual user resolution
	if len(conflicts) > 0 {
		return &MergeResult{
			Conflicts:   conflicts,
			MergedFiles: mergedFiles,
		}, ErrMergeConflict
	}

	// Clean merge: Create 2-parent merge commit
	mergedTreeHash, err := idx.WriteTree(r.Store)
	if err != nil {
		return nil, err
	}

	sig := DefaultSignature()
	msg := fmt.Sprintf("Merge branch '%s' into %s", target, currentBranch)
	mergeCommit := object.NewCommit(
		mergedTreeHash,
		[]object.Hash{headHash, incomingHash},
		sig,
		sig,
		msg,
	)

	mergeCommitHash, err := r.Store.Write(mergeCommit)
	if err != nil {
		return nil, err
	}

	if err := refMgr.UpdateHead(mergeCommitHash); err != nil {
		return nil, err
	}

	return &MergeResult{
		MergeCommit: mergeCommitHash,
		MergedFiles: mergedFiles,
	}, nil
}
