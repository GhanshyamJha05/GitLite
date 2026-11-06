package repository

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gitlite/internal/diff"
	"gitlite/internal/object"
	"gitlite/internal/refs"
)

// DiffWorkingTreeAgainstIndex compares files in the physical working tree against the staged index.
func (r *Repository) DiffWorkingTreeAgainstIndex() (string, error) {
	idx, err := r.LoadIndex()
	if err != nil {
		return "", fmt.Errorf("loading index: %w", err)
	}

	var sb strings.Builder

	for _, entry := range idx.SortedEntries() {
		fullPath := filepath.Join(r.WorkTree, filepath.FromSlash(entry.Path))

		blob, err := r.Store.ReadBlob(entry.Hash)
		if err != nil {
			return "", fmt.Errorf("reading staged blob %s: %w", entry.Hash, err)
		}
		stagedContent := string(blob.Data)

		workData, err := os.ReadFile(fullPath)
		if err != nil {
			if os.IsNotExist(err) {
				// File deleted in working tree
				u := diff.UnifiedDiff(entry.Path, entry.Path, stagedContent, "")
				sb.WriteString(u)
				continue
			}
			return "", fmt.Errorf("reading working tree file %s: %w", fullPath, err)
		}

		workContent := string(workData)
		u := diff.UnifiedDiff(entry.Path, entry.Path, stagedContent, workContent)
		sb.WriteString(u)
	}

	return sb.String(), nil
}

// DiffIndexAgainstHead compares staged index entries against the current HEAD commit tree.
func (r *Repository) DiffIndexAgainstHead() (string, error) {
	idx, err := r.LoadIndex()
	if err != nil {
		return "", fmt.Errorf("loading index: %w", err)
	}

	headHash, _, err := r.Refs().ResolveHead()
	headFiles := make(map[string]object.TreeEntry)
	if err == nil && headHash != "" {
		headCommit, err := r.Store.ReadCommit(headHash)
		if err != nil {
			return "", fmt.Errorf("reading HEAD commit: %w", err)
		}
		headFiles, err = FlattenTree(r.Store, headCommit.TreeHash)
		if err != nil {
			return "", fmt.Errorf("flattening HEAD tree: %w", err)
		}
	} else if err != refs.ErrRefNotFound {
		return "", fmt.Errorf("resolving HEAD: %w", err)
	}

	var sb strings.Builder

	// Check files in index vs HEAD
	for _, entry := range idx.SortedEntries() {
		blob, err := r.Store.ReadBlob(entry.Hash)
		if err != nil {
			return "", fmt.Errorf("reading staged blob %s: %w", entry.Hash, err)
		}
		stagedContent := string(blob.Data)

		headEntry, inHead := headFiles[entry.Path]
		headContent := ""
		if inHead {
			headBlob, err := r.Store.ReadBlob(headEntry.Hash)
			if err != nil {
				return "", fmt.Errorf("reading head blob %s: %w", headEntry.Hash, err)
			}
			headContent = string(headBlob.Data)
		}

		u := diff.UnifiedDiff(entry.Path, entry.Path, headContent, stagedContent)
		sb.WriteString(u)
	}

	// Check files in HEAD that are removed from index
	for path, headEntry := range headFiles {
		if _, inIndex := idx.Get(path); !inIndex {
			headBlob, err := r.Store.ReadBlob(headEntry.Hash)
			if err != nil {
				return "", fmt.Errorf("reading head blob %s: %w", headEntry.Hash, err)
			}
			u := diff.UnifiedDiff(path, path, string(headBlob.Data), "")
			sb.WriteString(u)
		}
	}

	return sb.String(), nil
}

// DiffCommits compares two arbitrary commits.
func (r *Repository) DiffCommits(hashA, hashB object.Hash) (string, error) {
	commitA, err := r.Store.ReadCommit(hashA)
	if err != nil {
		return "", fmt.Errorf("reading commit A %s: %w", hashA, err)
	}
	commitB, err := r.Store.ReadCommit(hashB)
	if err != nil {
		return "", fmt.Errorf("reading commit B %s: %w", hashB, err)
	}

	filesA, err := FlattenTree(r.Store, commitA.TreeHash)
	if err != nil {
		return "", err
	}
	filesB, err := FlattenTree(r.Store, commitB.TreeHash)
	if err != nil {
		return "", err
	}

	var sb strings.Builder

	// All files in B compared to A
	for path, entryB := range filesB {
		blobB, err := r.Store.ReadBlob(entryB.Hash)
		if err != nil {
			return "", err
		}
		bContent := string(blobB.Data)

		aContent := ""
		if entryA, inA := filesA[path]; inA {
			blobA, err := r.Store.ReadBlob(entryA.Hash)
			if err != nil {
				return "", err
			}
			aContent = string(blobA.Data)
		}

		u := diff.UnifiedDiff(path, path, aContent, bContent)
		sb.WriteString(u)
	}

	// Files deleted in B (present in A, absent in B)
	for path, entryA := range filesA {
		if _, inB := filesB[path]; !inB {
			blobA, err := r.Store.ReadBlob(entryA.Hash)
			if err != nil {
				return "", err
			}
			u := diff.UnifiedDiff(path, path, string(blobA.Data), "")
			sb.WriteString(u)
		}
	}

	return sb.String(), nil
}
