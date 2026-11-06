package repository

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"gitlite/internal/object"
)

// FileStatus represents the working-tree vs index state of a single file.
type FileStatus struct {
	Path     string
	Staged   bool
	Modified bool
}

// StatusResult encapsulates the categorization of repository state.
type StatusResult struct {
	StagedNew      []string
	StagedModified []string
	StagedDeleted  []string
	Untracked      []string
	Modified       []string
	Deleted        []string
}

// Status compares the working tree against the index.
func (r *Repository) Status() (*StatusResult, error) {
	idx, err := r.LoadIndex()
	if err != nil {
		return nil, fmt.Errorf("loading index: %w", err)
	}

	result := &StatusResult{}
	workTreeFiles := make(map[string]os.FileInfo)

	// Scan working tree
	err = filepath.WalkDir(r.WorkTree, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if d.IsDir() {
			if d.Name() == DotGitLite || d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}

		relPath, err := filepath.Rel(r.WorkTree, path)
		if err != nil {
			return err
		}
		normPath := filepath.ToSlash(relPath)

		info, err := d.Info()
		if err != nil {
			return err
		}
		workTreeFiles[normPath] = info
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walking working tree: %w", err)
	}

	// 1. Detect untracked files
	for path := range workTreeFiles {
		if _, staged := idx.Get(path); !staged {
			result.Untracked = append(result.Untracked, path)
		}
	}

	// 2. Detect modified or deleted files in working tree compared to index
	for path, entry := range idx.Entries {
		info, exists := workTreeFiles[path]
		if !exists {
			result.Deleted = append(result.Deleted, path)
			continue
		}

		// Fast change detection check (size & mod time)
		if info.Size() != entry.Size || info.ModTime().UnixNano() != entry.ModTimeNano {
			// Content might have changed; hash to confirm
			fullPath := filepath.Join(r.WorkTree, filepath.FromSlash(path))
			data, err := os.ReadFile(fullPath)
			if err != nil {
				continue
			}
			blob := object.NewBlob(data)
			canonicalBytes := object.EncodeCanonical(blob)
			h := object.ComputeHash(canonicalBytes)
			if h != entry.Hash {
				result.Modified = append(result.Modified, path)
			}
		}
	}

	// Staged items currently in the index (until Phase 4 commit diffing separates staged new vs modified vs HEAD)
	for _, entry := range idx.SortedEntries() {
		result.StagedNew = append(result.StagedNew, entry.Path)
	}

	sort.Strings(result.Untracked)
	sort.Strings(result.Modified)
	sort.Strings(result.Deleted)
	sort.Strings(result.StagedNew)

	return result, nil
}
