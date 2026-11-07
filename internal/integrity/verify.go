package integrity

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"gitlite/internal/object"
	"gitlite/internal/repository"
)

// Issue represents a corruption or missing reference discovered during verification.
type Issue struct {
	Level   string // "ERROR" or "WARNING"
	Message string
}

// Report encapsulates the result of an integrity check (fsck).
type Report struct {
	TotalObjects    int
	ValidObjects    int
	CorruptedCount  int
	DanglingObjects []object.Hash
	Issues          []Issue
}

// Fsck audits the entire repository: objects, references, connectivity, and dangling objects.
func Fsck(repo *repository.Repository) (*Report, error) {
	report := &Report{}

	objectsDir := filepath.Join(repo.GitLiteDir, "objects")
	if _, err := os.Stat(objectsDir); os.IsNotExist(err) {
		return report, nil
	}

	allObjects := make(map[object.Hash]object.Type)

	// 1. Audit all stored objects: decompression, hash match, canonical parsing
	err := filepath.WalkDir(objectsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		rel, err := filepath.Rel(objectsDir, path)
		if err != nil {
			return err
		}
		// Expected relative path: "ab/cdef..."
		norm := filepath.ToSlash(rel)

		cleanHex := ""
		for _, r := range norm {
			if r != '/' && r != '\\' {
				cleanHex += string(r)
			}
		}

		h := object.Hash(cleanHex)
		if err := h.Validate(); err != nil {
			report.Issues = append(report.Issues, Issue{
				Level:   "WARNING",
				Message: fmt.Sprintf("stray file in object store: %s", path),
			})
			return nil
		}

		report.TotalObjects++

		objType, _, err := repo.Store.Read(h)
		if err != nil {
			report.CorruptedCount++
			report.Issues = append(report.Issues, Issue{
				Level:   "ERROR",
				Message: fmt.Sprintf("corrupt object %s: %v", h, err),
			})
			return nil
		}

		report.ValidObjects++
		allObjects[h] = objType
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("scanning object store: %w", err)
	}

	// 2. Reachability Graph Traversal: Start from all refs
	reachable := make(map[object.Hash]bool)

	refMgr := repo.Refs()
	branches, err := refMgr.ListBranches()
	if err != nil {
		return nil, err
	}

	// Mark reachability starting from a tree
	var markTree func(treeHash object.Hash)
	markTree = func(treeHash object.Hash) {
		if reachable[treeHash] {
			return
		}
		reachable[treeHash] = true

		tree, err := repo.Store.ReadTree(treeHash)
		if err != nil {
			report.Issues = append(report.Issues, Issue{
				Level:   "ERROR",
				Message: fmt.Sprintf("missing or invalid tree %s: %v", treeHash, err),
			})
			return
		}

		for _, entry := range tree.Entries {
			if entry.Mode == object.ModeDirectory {
				markTree(entry.Hash)
			} else {
				reachable[entry.Hash] = true
				if !repo.Store.Has(entry.Hash) {
					report.Issues = append(report.Issues, Issue{
						Level:   "ERROR",
						Message: fmt.Sprintf("dangling tree entry %s in tree %s points to missing object %s", entry.Name, treeHash, entry.Hash),
					})
				}
			}
		}
	}

	// Mark reachability starting from a commit
	var markCommit func(commitHash object.Hash)
	markCommit = func(commitHash object.Hash) {
		if reachable[commitHash] {
			return
		}
		reachable[commitHash] = true

		commit, err := repo.Store.ReadCommit(commitHash)
		if err != nil {
			report.Issues = append(report.Issues, Issue{
				Level:   "ERROR",
				Message: fmt.Sprintf("missing or invalid commit %s: %v", commitHash, err),
			})
			return
		}

		// Traverse tree
		markTree(commit.TreeHash)

		// Traverse parents
		for _, p := range commit.Parents {
			markCommit(p)
		}
	}

	for _, b := range branches {
		if !repo.Store.Has(b.Commit) {
			report.Issues = append(report.Issues, Issue{
				Level:   "ERROR",
				Message: fmt.Sprintf("branch %s points to missing commit %s", b.Name, b.Commit),
			})
			continue
		}
		markCommit(b.Commit)
	}

	// Also check HEAD
	headHash, _, err := refMgr.ResolveHead()
	if err == nil && headHash != "" {
		markCommit(headHash)
	}

	// 3. Detect Dangling Objects
	for h := range allObjects {
		if !reachable[h] {
			report.DanglingObjects = append(report.DanglingObjects, h)
		}
	}

	sort.Slice(report.DanglingObjects, func(i, j int) bool {
		return report.DanglingObjects[i] < report.DanglingObjects[j]
	})

	return report, nil
}
