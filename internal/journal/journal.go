package journal

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

// ActionType identifies what operation was executed.
type ActionType string

const (
	ActionCommit   ActionType = "commit"
	ActionCheckout ActionType = "checkout"
	ActionMerge    ActionType = "merge"
	ActionAdd      ActionType = "add"
	ActionBranch   ActionType = "branch"
)

// JournalFile is the name of the journal state log inside .gitlite.
const JournalFile = "journal"

var (
	// ErrNoUndoAvailable is returned when there are no historical entries to undo.
	ErrNoUndoAvailable = errors.New("no operations available to undo")
)

// StateSnapshot captures the repository state before an operation takes place.
type StateSnapshot struct {
	Timestamp   time.Time   `json:"timestamp"`
	Action      ActionType  `json:"action"`
	Description string      `json:"description"`
	HeadRef     string      `json:"head_ref"`    // e.g. "refs/heads/main" or detached commit hash
	HeadCommit  object.Hash `json:"head_commit"` // commit hash at HEAD before action
	IndexData   []byte      `json:"index_data"`  // raw index file bytes
}

// Log represents the append-only journal of state snapshots.
type Log struct {
	Snapshots []StateSnapshot `json:"snapshots"`
}

// Manager handles recording and popping operations from the journal.
type Manager struct {
	journalPath string
}

// NewManager creates a journal manager for the specified gitliteDir.
func NewManager(gitliteDir string) *Manager {
	return &Manager{
		journalPath: filepath.Join(gitliteDir, JournalFile),
	}
}

// Load reads and parses the journal file.
func (m *Manager) Load() (*Log, error) {
	data, err := os.ReadFile(m.journalPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &Log{Snapshots: make([]StateSnapshot, 0)}, nil
		}
		return nil, fmt.Errorf("reading journal: %w", err)
	}

	var l Log
	if len(data) == 0 {
		return &Log{Snapshots: make([]StateSnapshot, 0)}, nil
	}
	if err := json.Unmarshal(data, &l); err != nil {
		return nil, fmt.Errorf("decoding journal: %w", err)
	}
	return &l, nil
}

// Record appends a new state snapshot to the journal.
func (m *Manager) Record(snapshot StateSnapshot) error {
	log, err := m.Load()
	if err != nil {
		return err
	}

	snapshot.Timestamp = time.Now()
	log.Snapshots = append(log.Snapshots, snapshot)

	data, err := json.MarshalIndent(log, "", "  ")
	if err != nil {
		return fmt.Errorf("serializing journal: %w", err)
	}

	return util.AtomicWriteFile(m.journalPath, data, 0644)
}

// PopLast removes and returns the most recent state snapshot from the journal.
func (m *Manager) PopLast() (*StateSnapshot, error) {
	log, err := m.Load()
	if err != nil {
		return nil, err
	}

	if len(log.Snapshots) == 0 {
		return nil, ErrNoUndoAvailable
	}

	lastIdx := len(log.Snapshots) - 1
	last := log.Snapshots[lastIdx]
	log.Snapshots = log.Snapshots[:lastIdx]

	data, err := json.MarshalIndent(log, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("serializing journal: %w", err)
	}

	if err := util.AtomicWriteFile(m.journalPath, data, 0644); err != nil {
		return nil, err
	}

	return &last, nil
}
