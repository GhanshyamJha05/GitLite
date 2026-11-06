package integration_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gitlite/internal/repository"
)

func TestAddAndStatus(t *testing.T) {
	tempDir := t.TempDir()

	// Initialize repository
	_, err := executeCommand("init", tempDir)
	if err != nil {
		t.Fatalf("init failed: %v", err)
	}

	// Change working directory to tempDir for command testing
	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("chdir failed: %v", err)
	}

	// Initial clean status
	out, err := executeCommand("status")
	if err != nil {
		t.Fatalf("status failed: %v", err)
	}
	if !strings.Contains(out, "nothing to commit, working tree clean") {
		t.Errorf("expected clean working tree, got: %s", out)
	}

	// Create test files
	fileA := filepath.Join(tempDir, "file_a.txt")
	if err := os.WriteFile(fileA, []byte("hello file a"), 0644); err != nil {
		t.Fatalf("writing fileA: %v", err)
	}

	subDir := filepath.Join(tempDir, "pkg")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("creating pkg: %v", err)
	}
	fileB := filepath.Join(subDir, "file_b.txt")
	if err := os.WriteFile(fileB, []byte("hello nested file b"), 0644); err != nil {
		t.Fatalf("writing fileB: %v", err)
	}

	// Check untracked files
	out, err = executeCommand("status")
	if err != nil {
		t.Fatalf("status failed: %v", err)
	}
	if !strings.Contains(out, "file_a.txt") || !strings.Contains(out, "pkg/file_b.txt") {
		t.Errorf("expected untracked files, got: %s", out)
	}

	// Stage fileA
	_, err = executeCommand("add", "file_a.txt")
	if err != nil {
		t.Fatalf("add file_a.txt failed: %v", err)
	}

	// Check status: file_a is staged, file_b is untracked
	out, err = executeCommand("status")
	if err != nil {
		t.Fatalf("status failed: %v", err)
	}
	if !strings.Contains(out, "Changes to be committed:") || !strings.Contains(out, "new file:   file_a.txt") {
		t.Errorf("expected file_a to be staged, got: %s", out)
	}
	if !strings.Contains(out, "Untracked files:") || !strings.Contains(out, "pkg/file_b.txt") {
		t.Errorf("expected pkg/file_b.txt to be untracked, got: %s", out)
	}

	// Stage all with add .
	_, err = executeCommand("add", ".")
	if err != nil {
		t.Fatalf("add . failed: %v", err)
	}

	// Check status: all files staged
	out, err = executeCommand("status")
	if err != nil {
		t.Fatalf("status failed: %v", err)
	}
	if strings.Contains(out, "Untracked files:") {
		t.Errorf("expected no untracked files, got: %s", out)
	}
	if !strings.Contains(out, "new file:   pkg/file_b.txt") {
		t.Errorf("expected pkg/file_b.txt to be staged, got: %s", out)
	}

	// Verify that index can construct root tree in object store
	repo, err := repository.Discover(tempDir)
	if err != nil {
		t.Fatalf("Discover failed: %v", err)
	}

	idx, err := repo.LoadIndex()
	if err != nil {
		t.Fatalf("LoadIndex failed: %v", err)
	}

	rootTreeHash, err := idx.WriteTree(repo.Store)
	if err != nil {
		t.Fatalf("idx.WriteTree failed: %v", err)
	}

	if !repo.Store.Has(rootTreeHash) {
		t.Errorf("root tree was not stored in CAS")
	}
}
