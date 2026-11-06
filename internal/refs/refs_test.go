package refs_test

import (
	"testing"

	"gitlite/internal/object"
	"gitlite/internal/refs"
	"gitlite/internal/repository"
)

func TestRefResolutionAndAtomicUpdate(t *testing.T) {
	tempDir := t.TempDir()
	repo, err := repository.Init(tempDir)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	mgr := refs.NewManager(repo.GitLiteDir)

	// Unborn branch main
	headHash, branch, err := mgr.ResolveHead()
	if err != refs.ErrRefNotFound {
		t.Fatalf("expected ErrRefNotFound on unborn branch, got %v", err)
	}
	if branch != "main" {
		t.Errorf("expected branch main, got %s", branch)
	}
	if headHash != "" {
		t.Errorf("expected empty hash on unborn branch, got %s", headHash)
	}

	// Update HEAD with initial commit
	dummyCommit := object.ComputeHash([]byte("commit 1"))
	if err := mgr.UpdateHead(dummyCommit); err != nil {
		t.Fatalf("UpdateHead failed: %v", err)
	}

	// Resolve HEAD again
	headHash, branch, err = mgr.ResolveHead()
	if err != nil {
		t.Fatalf("ResolveHead after commit failed: %v", err)
	}
	if headHash != dummyCommit {
		t.Errorf("expected %s, got %s", dummyCommit, headHash)
	}
	if branch != "main" {
		t.Errorf("expected branch main, got %s", branch)
	}

	// Advance branch with second commit
	commit2 := object.ComputeHash([]byte("commit 2"))
	if err := mgr.UpdateHead(commit2); err != nil {
		t.Fatalf("second UpdateHead failed: %v", err)
	}

	headHash, _, err = mgr.ResolveHead()
	if err != nil || headHash != commit2 {
		t.Fatalf("expected HEAD to advance to %s, got %s", commit2, headHash)
	}
}
