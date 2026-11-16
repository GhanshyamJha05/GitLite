package repository

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gitlite/internal/index"
	"gitlite/internal/journal"
	"gitlite/internal/object"
	"gitlite/internal/util"
)

// ResetMode represents the behavior mode for gitlite reset.
type ResetMode int

const (
	// ResetSoft moves HEAD only; preserves index and working tree.
	ResetSoft ResetMode = iota
	// ResetMixed moves HEAD and updates index to match; preserves working tree changes.
	ResetMixed
	// ResetHard moves HEAD, updates index, and discards all working tree modifications.
	ResetHard
)

// Reset updates HEAD to targetCommit according to the specified ResetMode.
func (r *Repository) Reset(targetCommit object.Hash, mode ResetMode) error {
	if err := targetCommit.Validate(); err != nil {
		return fmt.Errorf("validating target commit: %w", err)
	}

	commitObj, err := r.Store.ReadCommit(targetCommit)
	if err != nil {
		return fmt.Errorf("reading target commit %s: %w", targetCommit, err)
	}

	// 1. Snapshot state for undoability & record current HEAD before moving
	oldHeadH, _, _ := r.Refs().ResolveHead()

	modeDesc := "mixed"
	switch mode {
	case ResetSoft:
		modeDesc = "soft"
	case ResetHard:
		modeDesc = "hard"
	}
	snap, err := r.SnapshotState(journal.ActionReset, fmt.Sprintf("reset --%s %s", modeDesc, targetCommit[:min(8, len(targetCommit))]))
	if err == nil {
		_ = r.Journal().Record(*snap)
	}

	// 2. Move HEAD / active branch ref
	if err := r.Refs().UpdateHead(targetCommit); err != nil {
		return fmt.Errorf("updating HEAD: %w", err)
	}

	if mode == ResetSoft {
		// Soft reset only moves HEAD; index and working tree remain untouched
		return nil
	}

	// 3. For Mixed and Hard: update index from target commit's tree
	targetFiles, err := FlattenTree(r.Store, commitObj.TreeHash)
	if err != nil {
		return fmt.Errorf("flattening target tree: %w", err)
	}

	currIdx, err := r.LoadIndex()
	if err != nil {
		currIdx = index.New()
	}

	newIdx := index.New()
	for relPath, entry := range targetFiles {
		var size int64
		var mtimeNano int64

		fullPath := filepath.Join(r.WorkTree, filepath.FromSlash(relPath))
		if info, err := os.Stat(fullPath); err == nil {
			size = info.Size()
			mtimeNano = info.ModTime().UnixNano()
		}

		newIdx.Add(index.Entry{
			Path:        relPath,
			Hash:        entry.Hash,
			Mode:        entry.Mode,
			Size:        size,
			ModTimeNano: mtimeNano,
		})
	}

	// Persist updated index
	idxPath := r.IndexPath()
	newIdxData, err := json.Marshal(newIdx)
	if err != nil {
		return fmt.Errorf("marshaling new index: %w", err)
	}
	if err := util.AtomicWriteFile(idxPath, newIdxData, 0644); err != nil {
		return fmt.Errorf("writing updated index: %w", err)
	}

	if mode == ResetMixed {
		// Working tree is untouched
		return nil
	}

	// 4. Hard reset: clean untracked/deleted files from old index, old HEAD tree, and overwrite working tree
	oldHeadFiles := make(map[string]bool)
	for relPath := range currIdx.Entries {
		oldHeadFiles[relPath] = true
	}
	if oldHeadH != "" {
		if oldCommit, err := r.Store.ReadCommit(oldHeadH); err == nil {
			if oldTree, err := FlattenTree(r.Store, oldCommit.TreeHash); err == nil {
				for relPath := range oldTree {
					oldHeadFiles[relPath] = true
				}
			}
		}
	}

	for relPath := range oldHeadFiles {
		if _, existsInTarget := targetFiles[relPath]; !existsInTarget {
			fullPath := filepath.Join(r.WorkTree, filepath.FromSlash(relPath))
			_ = os.Remove(fullPath)
			// Clean empty parent directories
			normWorkTree := filepath.Clean(r.WorkTree)
			parent := filepath.Dir(fullPath)
			for {
				cleanParent := filepath.Clean(parent)
				if cleanParent == normWorkTree || cleanParent == "." || !strings.HasPrefix(cleanParent, normWorkTree) {
					break
				}
				if err := os.Remove(cleanParent); err != nil {
					break
				}
				parent = filepath.Dir(cleanParent)
			}
		}
	}

	for relPath, entry := range targetFiles {
		fullPath := filepath.Join(r.WorkTree, filepath.FromSlash(relPath))
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			return fmt.Errorf("creating directory for %s: %w", relPath, err)
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
	}

	return nil
}
