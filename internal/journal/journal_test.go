package journal

import (
	"testing"

	"gitlite/internal/object"
)

func TestJournalRecordAndPop(t *testing.T) {
	tempDir := t.TempDir()
	mgr := NewManager(tempDir)

	// Initially empty
	_, err := mgr.PopLast()
	if err != ErrNoUndoAvailable {
		t.Fatalf("expected ErrNoUndoAvailable, got: %v", err)
	}

	snap1 := StateSnapshot{
		Action:      ActionCommit,
		Description: "commit first",
		HeadRef:     "refs/heads/main",
		HeadCommit:  object.Hash("1111111111111111111111111111111111111111111111111111111111111111"),
		IndexData:   []byte("index data 1"),
	}

	snap2 := StateSnapshot{
		Action:      ActionCheckout,
		Description: "checkout feature",
		HeadRef:     "refs/heads/feature",
		HeadCommit:  object.Hash("2222222222222222222222222222222222222222222222222222222222222222"),
		IndexData:   []byte("index data 2"),
	}

	if err := mgr.Record(snap1); err != nil {
		t.Fatalf("record snap1 failed: %v", err)
	}
	if err := mgr.Record(snap2); err != nil {
		t.Fatalf("record snap2 failed: %v", err)
	}

	popped2, err := mgr.PopLast()
	if err != nil {
		t.Fatalf("pop last failed: %v", err)
	}
	if popped2.Description != "checkout feature" {
		t.Errorf("expected snap2 description, got: %s", popped2.Description)
	}

	popped1, err := mgr.PopLast()
	if err != nil {
		t.Fatalf("pop 1 failed: %v", err)
	}
	if popped1.Description != "commit first" {
		t.Errorf("expected snap1 description, got: %s", popped1.Description)
	}

	// Should be empty again
	_, err = mgr.PopLast()
	if err != ErrNoUndoAvailable {
		t.Fatalf("expected empty journal error, got: %v", err)
	}
}
