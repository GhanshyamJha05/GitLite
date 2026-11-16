package repository

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"gitlite/internal/object"
)

func TestStashWorkflow(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "gitlite-stash-test-*")
	if err != nil {
		t.Fatalf("temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	repo, err := Init(tempDir)
	if err != nil {
		t.Fatalf("init repo: %v", err)
	}

	sig := object.Signature{Name: "Tester", Email: "test@example.com", When: time.Now()}

	f1 := filepath.Join(tempDir, "file1.txt")
	_ = os.WriteFile(f1, []byte("committed version\n"), 0644)
	_ = repo.Add(f1)
	_, err = repo.Commit("initial commit", sig, sig)
	if err != nil {
		t.Fatalf("initial commit: %v", err)
	}

	// Make uncommitted changes
	_ = os.WriteFile(f1, []byte("uncommitted edits\n"), 0644)
	_ = repo.Add(f1)

	// Save to stash
	stashEntry, err := repo.StashSave("test work in progress")
	if err != nil {
		t.Fatalf("StashSave failed: %v", err)
	}
	if stashEntry.Description != "test work in progress" {
		t.Errorf("unexpected stash description: %s", stashEntry.Description)
	}

	// Working tree should be reverted to committed version
	content, _ := os.ReadFile(f1)
	if string(content) != "committed version\n" {
		t.Fatalf("expected file to be reverted to committed version, got %q", string(content))
	}

	// List stashes
	stashes, err := repo.StashList()
	if err != nil {
		t.Fatalf("StashList failed: %v", err)
	}
	if len(stashes) != 1 {
		t.Fatalf("expected 1 stash entry, got %d", len(stashes))
	}

	// Pop stash
	popped, err := repo.StashPop()
	if err != nil {
		t.Fatalf("StashPop failed: %v", err)
	}
	if popped.Description != "test work in progress" {
		t.Errorf("unexpected popped description: %s", popped.Description)
	}

	// Working tree should have uncommitted edits restored
	content, _ = os.ReadFile(f1)
	if string(content) != "uncommitted edits\n" {
		t.Fatalf("expected restored edits, got %q", string(content))
	}

	// Stash list should now be empty
	stashes, _ = repo.StashList()
	if len(stashes) != 0 {
		t.Fatalf("expected 0 stash entries after pop, got %d", len(stashes))
	}
}
