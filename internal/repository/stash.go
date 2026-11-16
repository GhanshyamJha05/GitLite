package repository

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gitlite/internal/object"
	"gitlite/internal/util"
)

// StashEntry represents a single stashed state.
type StashEntry struct {
	ID          string                  `json:"id"`
	Description string                  `json:"description"`
	Branch      string                  `json:"branch"`
	HeadCommit  object.Hash             `json:"head_commit"`
	Timestamp   time.Time               `json:"timestamp"`
	Files       map[string]StashFile    `json:"files"`
	IndexData   []byte                  `json:"index_data,omitempty"`
}

// StashFile stores the blob hash and file mode of a stashed file.
type StashFile struct {
	Hash object.Hash     `json:"hash"`
	Mode object.FileMode `json:"mode"`
}

// StashLog stores the list of stashes in LIFO order (stack).
type StashLog struct {
	Entries []StashEntry `json:"entries"`
}

const (
	// StashFileLocation is the relative path inside .gitlite where stashes are tracked.
	StashFileLocation = "stash"
)

var (
	// ErrNoStashes indicates there are no stashes to pop or show.
	ErrNoStashes = errors.New("no stash entries found")
	// ErrNoLocalChangesToStash is returned when working directory and index are clean.
	ErrNoLocalChangesToStash = errors.New("no local changes to save")
)

// LoadStashes reads the stash stack from .gitlite/stash.
func (r *Repository) LoadStashes() (*StashLog, error) {
	stashPath := filepath.Join(r.GitLiteDir, StashFileLocation)
	data, err := os.ReadFile(stashPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &StashLog{Entries: []StashEntry{}}, nil
		}
		return nil, fmt.Errorf("reading stash log: %w", err)
	}

	var sl StashLog
	if err := json.Unmarshal(data, &sl); err != nil {
		return nil, fmt.Errorf("decoding stash log: %w", err)
	}
	return &sl, nil
}

// SaveStashes writes the stash stack to .gitlite/stash atomically.
func (r *Repository) SaveStashes(sl *StashLog) error {
	stashPath := filepath.Join(r.GitLiteDir, StashFileLocation)
	data, err := json.MarshalIndent(sl, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding stash log: %w", err)
	}
	data = append(data, '\n')
	return util.AtomicWriteFile(stashPath, data, 0644)
}

// StashSave saves current uncommitted working tree and staged index changes into a stash entry,
// then restores the working tree and index to match HEAD.
func (r *Repository) StashSave(message string) (*StashEntry, error) {
	status, err := r.Status()
	if err != nil {
		return nil, fmt.Errorf("checking status: %w", err)
	}

	hasChanges := len(status.StagedNew) > 0 || len(status.StagedModified) > 0 || len(status.StagedDeleted) > 0 ||
		len(status.Modified) > 0 || len(status.Deleted) > 0
	if !hasChanges {
		return nil, ErrNoLocalChangesToStash
	}

	headH, branch, _ := r.Refs().ResolveHead()
	if message == "" {
		branchDesc := branch
		if branchDesc == "" {
			branchDesc = "(no branch)"
		}
		headShort := string(headH)
		if len(headShort) > 8 {
			headShort = headShort[:8]
		}
		message = fmt.Sprintf("WIP on %s: %s", branchDesc, headShort)
	}

	currIdx, err := r.LoadIndex()
	if err != nil {
		return nil, fmt.Errorf("loading index: %w", err)
	}

	// Capture modified/staged files into CAS blobs
	stashedFiles := make(map[string]StashFile)

	// All files currently tracked in the index or modified in the working tree
	for relPath, entry := range currIdx.Entries {
		fullPath := filepath.Join(r.WorkTree, filepath.FromSlash(relPath))
		if data, err := os.ReadFile(fullPath); err == nil {
			blob := object.NewBlob(data)
			bHash, err := r.Store.Write(blob)
			if err != nil {
				return nil, fmt.Errorf("saving blob for %s: %w", relPath, err)
			}
			stashedFiles[relPath] = StashFile{
				Hash: bHash,
				Mode: entry.Mode,
			}
		}
	}

	idxBytes, _ := os.ReadFile(r.IndexPath())

	sl, err := r.LoadStashes()
	if err != nil {
		sl = &StashLog{}
	}

	entry := StashEntry{
		ID:          fmt.Sprintf("stash@{%d}", len(sl.Entries)),
		Description: message,
		Branch:      branch,
		HeadCommit:  headH,
		Timestamp:   time.Now(),
		Files:       stashedFiles,
		IndexData:   idxBytes,
	}

	// Prepend to stack
	sl.Entries = append([]StashEntry{entry}, sl.Entries...)
	if err := r.SaveStashes(sl); err != nil {
		return nil, fmt.Errorf("saving stash stack: %w", err)
	}

	// Reset working tree and index to HEAD
	if headH != "" {
		if err := r.Reset(headH, ResetHard); err != nil {
			return nil, fmt.Errorf("resetting working tree after stash: %w", err)
		}
	}

	return &entry, nil
}

// StashPop restores the top stash entry and removes it from the stack.
func (r *Repository) StashPop() (*StashEntry, error) {
	sl, err := r.LoadStashes()
	if err != nil {
		return nil, err
	}
	if len(sl.Entries) == 0 {
		return nil, ErrNoStashes
	}

	top := sl.Entries[0]
	sl.Entries = sl.Entries[1:]

	// Restore files to working tree
	for relPath, sf := range top.Files {
		fullPath := filepath.Join(r.WorkTree, filepath.FromSlash(relPath))
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			return nil, fmt.Errorf("creating dir: %w", err)
		}
		blob, err := r.Store.ReadBlob(sf.Hash)
		if err != nil {
			return nil, fmt.Errorf("reading stashed blob %s: %w", sf.Hash, err)
		}
		perm := os.FileMode(0644)
		if sf.Mode == object.ModeExecutable {
			perm = 0755
		}
		if err := os.WriteFile(fullPath, blob.Data, perm); err != nil {
			return nil, fmt.Errorf("restoring stashed file %s: %w", fullPath, err)
		}
	}

	// Restore index if present
	if len(top.IndexData) > 0 {
		_ = util.AtomicWriteFile(r.IndexPath(), top.IndexData, 0644)
	}

	if err := r.SaveStashes(sl); err != nil {
		return nil, fmt.Errorf("saving stash stack: %w", err)
	}

	return &top, nil
}

// StashList returns all stashed states.
func (r *Repository) StashList() ([]StashEntry, error) {
	sl, err := r.LoadStashes()
	if err != nil {
		return nil, err
	}
	return sl.Entries, nil
}
