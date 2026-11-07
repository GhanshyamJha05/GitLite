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

	// Test branch sanitization
	invalidNames := []string{
		"../escape",
		"feat/sub",
		"feat\\sub",
		"branch with space",
		"-leadingdash",
		".hidden",
		"branch..double",
		"",
	}

	for _, badName := range invalidNames {
		if err := mgr.CreateBranch(badName, commit2); err == nil {
			t.Errorf("expected error creating branch with invalid name %q, got nil", badName)
		}
	}

	// Valid branch name
	if err := mgr.CreateBranch("feature-123", commit2); err != nil {
		t.Fatalf("expected valid branch creation to succeed, got %v", err)
	}

	// Atomic SetHeadBranch and SetHeadDetached
	if err := mgr.SetHeadBranch("feature-123"); err != nil {
		t.Fatalf("SetHeadBranch failed: %v", err)
	}
	_, currentBranch, _ := mgr.ResolveHead()
	if currentBranch != "feature-123" {
		t.Errorf("expected branch feature-123, got %s", currentBranch)
	}

	if err := mgr.SetHeadDetached(commit2); err != nil {
		t.Fatalf("SetHeadDetached failed: %v", err)
	}
	h, branch, err := mgr.ResolveHead()
	if err != nil || branch != "" || h != commit2 {
		t.Errorf("expected detached HEAD pointing to %s, got hash=%s branch=%s", commit2, h, branch)
	}
}
