package integration_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMergeCleanAndFastForward(t *testing.T) {
	tempDir := t.TempDir()

	// Initialize repo
	_, err := executeCommand("init", tempDir)
	if err != nil {
		t.Fatalf("init failed: %v", err)
	}

	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("chdir failed: %v", err)
	}

	// 1. Initial commit on main
	fileA := filepath.Join(tempDir, "shared.txt")
	if err := os.WriteFile(fileA, []byte("shared line 1\nshared line 2\n"), 0644); err != nil {
		t.Fatalf("writing shared: %v", err)
	}
	_, _ = executeCommand("add", "shared.txt")
	_, _ = executeCommand("commit", "-m", "init commit")

	// 2. Create feature branch and make non-conflicting commit
	outB, err := executeCommand("branch", "feature-a")
	if err != nil {
		t.Fatalf("create branch feature-a failed: %v, out: %s", err, outB)
	}
	outC, err := executeCommand("checkout", "feature-a")
	if err != nil {
		t.Fatalf("checkout feature-a failed: %v, out: %s", err, outC)
	}

	featFile := filepath.Join(tempDir, "feat_a.txt")
	if err := os.WriteFile(featFile, []byte("feature A content\n"), 0644); err != nil {
		t.Fatalf("writing feat_a: %v", err)
	}
	_, _ = executeCommand("add", "feat_a.txt")
	_, _ = executeCommand("commit", "-m", "feat A commit")

	// 3. Switch back to main; merge feature-a (Fast-forward scenario)
	_, _ = executeCommand("checkout", "main")
	ffOut, err := executeCommand("merge", "feature-a")
	if err != nil {
		t.Fatalf("fast-forward merge failed: %v", err)
	}
	if !strings.Contains(ffOut, "Fast-forward") {
		t.Errorf("expected Fast-forward output, got: %s", ffOut)
	}

	// Verify feat_a.txt now exists in main
	if _, err := os.Stat(featFile); os.IsNotExist(err) {
		t.Fatalf("expected feat_a.txt to exist after fast-forward merge")
	}

	// 4. Test 3-way clean merge:
	// Branch out to feature-b
	_, _ = executeCommand("branch", "feature-b")
	_, _ = executeCommand("checkout", "feature-b")
	featBFile := filepath.Join(tempDir, "feat_b.txt")
	_ = os.WriteFile(featBFile, []byte("feature B content\n"), 0644)
	_, _ = executeCommand("add", "feat_b.txt")
	_, _ = executeCommand("commit", "-m", "feat B commit")

	// Switch to main and make an independent commit on main
	_, _ = executeCommand("checkout", "main")
	mainOnly := filepath.Join(tempDir, "main_only.txt")
	_ = os.WriteFile(mainOnly, []byte("main independent work\n"), 0644)
	_, _ = executeCommand("add", "main_only.txt")
	_, _ = executeCommand("commit", "-m", "main branch independent commit")

	// 3-way merge feature-b into main
	mergeOut, err := executeCommand("merge", "feature-b")
	if err != nil {
		t.Fatalf("3-way merge failed: %v", err)
	}
	if !strings.Contains(mergeOut, "Merge made by the 'three-way' strategy") {
		t.Errorf("expected 3-way merge output, got: %s", mergeOut)
	}

	// Both main_only.txt and feat_b.txt must exist on main
	if _, err := os.Stat(mainOnly); os.IsNotExist(err) {
		t.Fatalf("main_only.txt missing after merge")
	}
	if _, err := os.Stat(featBFile); os.IsNotExist(err) {
		t.Fatalf("feat_b.txt missing after merge")
	}

	// Check that log shows a merge commit with 2 parents
	logOut, _ := executeCommand("log")
	if !strings.Contains(logOut, "Merge:") {
		t.Errorf("expected merge commit parent info in log, got:\n%s", logOut)
	}
}

func TestMergeConflictDetection(t *testing.T) {
	tempDir := t.TempDir()

	_, _ = executeCommand("init", tempDir)
	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	_ = os.Chdir(tempDir)

	confFile := filepath.Join(tempDir, "conflict.txt")
	_ = os.WriteFile(confFile, []byte("base line\n"), 0644)
	_, _ = executeCommand("add", "conflict.txt")
	_, _ = executeCommand("commit", "-m", "base commit")

	// Branch branch-x
	_, _ = executeCommand("branch", "branch-x")
	_, _ = executeCommand("checkout", "branch-x")
	_ = os.WriteFile(confFile, []byte("branch-x edit\n"), 0644)
	_, _ = executeCommand("add", "conflict.txt")
	_, _ = executeCommand("commit", "-m", "branch-x commit")

	// Branch main edit
	_, _ = executeCommand("checkout", "main")
	_ = os.WriteFile(confFile, []byte("main branch conflicting edit\n"), 0644)
	_, _ = executeCommand("add", "conflict.txt")
	_, _ = executeCommand("commit", "-m", "main branch commit")

	// Merge branch-x into main -> CONFLICT
	mergeOut, err := executeCommand("merge", "branch-x")
	if err != nil {
		t.Fatalf("merge command error: %v", err)
	}

	if !strings.Contains(mergeOut, "CONFLICT (content): Merge conflict in conflict.txt") {
		t.Errorf("expected conflict notification, got: %s", mergeOut)
	}

	// Verify conflict markers are inside conflict.txt
	data, _ := os.ReadFile(confFile)
	content := string(data)
	if !strings.Contains(content, "<<<<<<< CURRENT") || !strings.Contains(content, "main branch conflicting edit") || !strings.Contains(content, "=======") || !strings.Contains(content, "branch-x edit") || !strings.Contains(content, ">>>>>>> INCOMING") {
		t.Errorf("conflict markers not properly rendered in file:\n%s", content)
	}
}
