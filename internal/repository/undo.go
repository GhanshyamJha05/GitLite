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

// Journal returns the journal manager bound to this repository.
func (r *Repository) Journal() *journal.Manager {
	return journal.NewManager(r.GitLiteDir)
}

// SnapshotState captures the current HEAD ref, commit hash, and index data.
func (r *Repository) SnapshotState(action journal.ActionType, description string) (*journal.StateSnapshot, error) {
	headPath := filepath.Join(r.GitLiteDir, HeadFile)
	headBytes, err := os.ReadFile(headPath)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("reading HEAD: %w", err)
	}
	headRef := strings.TrimSpace(string(headBytes))

	headCommit, _, _ := r.Refs().ResolveHead()

	idxPath := r.IndexPath()
	var idxBytes []byte
	if data, err := os.ReadFile(idxPath); err == nil {
		idxBytes = data
	}

	return &journal.StateSnapshot{
		Action:      action,
		Description: description,
		HeadRef:     headRef,
		HeadCommit:  headCommit,
		IndexData:   idxBytes,
	}, nil
}

// Undo restores the repository state to before the most recent recorded operation.
func (r *Repository) Undo() (*journal.StateSnapshot, error) {
	lastSnap, err := r.Journal().PopLast()
	if err != nil {
		return nil, err
	}

	// 1. Restore HEAD file exactly as it was
	headPath := filepath.Join(r.GitLiteDir, HeadFile)
	if lastSnap.HeadRef != "" {
		if err := util.AtomicWriteFile(headPath, []byte(lastSnap.HeadRef+"\n"), 0644); err != nil {
			return nil, fmt.Errorf("restoring HEAD: %w", err)
		}
	}

	// 2. If HeadRef pointed to a branch and we have a HeadCommit, restore the branch ref
	if strings.HasPrefix(lastSnap.HeadRef, "ref: ") {
		refPath := strings.TrimSpace(strings.TrimPrefix(lastSnap.HeadRef, "ref: "))
		targetRefAbs := filepath.Join(r.GitLiteDir, filepath.FromSlash(refPath))
		if lastSnap.HeadCommit != "" {
			refPayload := []byte(fmt.Sprintf("%s\n", lastSnap.HeadCommit))
			if err := util.AtomicWriteFile(targetRefAbs, refPayload, 0644); err != nil {
				return nil, fmt.Errorf("restoring branch ref %s: %w", refPath, err)
			}
		} else {
			// Unborn branch before the undone action: remove the newly created branch ref file
			_ = os.Remove(targetRefAbs)
		}
	}

	// 3. Restore the index
	idxPath := r.IndexPath()
	if len(lastSnap.IndexData) > 0 {
		if err := util.AtomicWriteFile(idxPath, lastSnap.IndexData, 0644); err != nil {
			return nil, fmt.Errorf("restoring index: %w", err)
		}
	} else {
		// Index was empty before
		_ = os.Remove(idxPath)
	}

	// 4. Restore working tree files and clean up deleted files
	// Read current files tracked in the repository before restoration
	currTrackedFiles := make(map[string]bool)
	if curIdx, err := r.LoadIndex(); err == nil {
		for p := range curIdx.Entries {
			currTrackedFiles[p] = true
		}
	}

	targetFiles := make(map[string]object.TreeEntry)
	if lastSnap.HeadCommit != "" {
		commitObj, err := r.Store.ReadCommit(lastSnap.HeadCommit)
		if err != nil {
			return nil, fmt.Errorf("reading restored commit %s: %w", lastSnap.HeadCommit, err)
		}
		tf, err := FlattenTree(r.Store, commitObj.TreeHash)
		if err != nil {
			return nil, fmt.Errorf("flattening restored tree: %w", err)
		}
		targetFiles = tf
	}

	// Also consider files in the restored index (for staged uncommitted changes)
	if len(lastSnap.IndexData) > 0 {
		var restoredIdx index.Index
		if err := json.Unmarshal(lastSnap.IndexData, &restoredIdx); err == nil {
			for p, ie := range restoredIdx.Entries {
				if _, exists := targetFiles[p]; !exists {
					targetFiles[p] = object.TreeEntry{
						Name: filepath.Base(p),
						Mode: ie.Mode,
						Hash: ie.Hash,
					}
				}
			}
		}
	}

	// A. Remove files that were in the working tree/index before undo but NOT in the target restored state
	for relPath := range currTrackedFiles {
		if _, exists := targetFiles[relPath]; !exists {
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

	// B. Restore all target files from CAS blobs into the working tree
	for relPath, entry := range targetFiles {
		fullPath := filepath.Join(r.WorkTree, filepath.FromSlash(relPath))
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			return nil, fmt.Errorf("creating directory for %s: %w", relPath, err)
		}
		blob, err := r.Store.ReadBlob(entry.Hash)
		if err != nil {
			return nil, fmt.Errorf("reading blob %s for %s: %w", entry.Hash, relPath, err)
		}
		perm := os.FileMode(0644)
		if entry.Mode == object.ModeExecutable {
			perm = 0755
		}
		if err := os.WriteFile(fullPath, blob.Data, perm); err != nil {
			return nil, fmt.Errorf("writing restored file %s: %w", relPath, err)
		}
	}

	return lastSnap, nil
}
