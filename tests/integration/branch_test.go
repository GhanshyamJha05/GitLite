package integration_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBranchAndCheckout(t *testing.T) {
	tempDir := t.TempDir()

	// 1. Initialize repo
	_, err := executeCommand("init", tempDir)
	if err != nil {
		t.Fatalf("init failed: %v", err)
	}

	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("chdir failed: %v", err)
	}

	// 2. Cannot branch before initial commit
	_, err = executeCommand("branch", "feature-1")
	if err == nil {
		t.Fatalf("expected branching from empty HEAD to fail")
	}

	// 3. Create initial commit on main
	fileA := filepath.Join(tempDir, "shared.txt")
	if err := os.WriteFile(fileA, []byte("main version"), 0644); err != nil {
		t.Fatalf("writing shared.txt: %v", err)
	}
	_, _ = executeCommand("add", "shared.txt")
	_, err = executeCommand("commit", "-m", "main: initial commit")
	if err != nil {
		t.Fatalf("initial commit failed: %v", err)
	}

	// 4. Create feature-1 branch
	_, err = executeCommand("branch", "feature-1")
	if err != nil {
		t.Fatalf("branch feature-1 failed: %v", err)
	}

	// 5. List branches
	branchListOut, err := executeCommand("branch")
	if err != nil {
		t.Fatalf("branch listing failed: %v", err)
	}
	if !strings.Contains(branchListOut, "main") || !strings.Contains(branchListOut, "feature-1") {
		t.Errorf("expected main and feature-1 in branch list, got: %s", branchListOut)
	}

	// 6. Switch to feature-1
	_, err = executeCommand("checkout", "feature-1")
	if err != nil {
		t.Fatalf("checkout feature-1 failed: %v", err)
	}

	// 7. Make a commit on feature-1
	featureFile := filepath.Join(tempDir, "feature.txt")
	if err := os.WriteFile(featureFile, []byte("feature 1 unique content"), 0644); err != nil {
		t.Fatalf("writing feature.txt: %v", err)
	}
	_, _ = executeCommand("add", "feature.txt")
	_, err = executeCommand("commit", "-m", "feat: added feature.txt")
	if err != nil {
		t.Fatalf("commit on feature-1 failed: %v", err)
	}

	// Verify feature.txt exists
	if _, err := os.Stat(featureFile); os.IsNotExist(err) {
		t.Fatalf("expected feature.txt to exist on feature-1")
	}

	// 8. Switch back to main using 'switch'
	_, err = executeCommand("switch", "main")
	if err != nil {
		t.Fatalf("switch main failed: %v", err)
	}

	// feature.txt must be removed because it does not exist on main!
	if _, err := os.Stat(featureFile); !os.IsNotExist(err) {
		t.Fatalf("expected feature.txt to be removed when switching to main")
	}

	// shared.txt must retain its main contents
	sharedData, _ := os.ReadFile(fileA)
	if string(sharedData) != "main version" {
		t.Errorf("expected 'main version', got: %s", string(sharedData))
	}

	// 9. Switch back to feature-1; feature.txt must be safely restored
	_, err = executeCommand("checkout", "feature-1")
	if err != nil {
		t.Fatalf("switch back to feature-1 failed: %v", err)
	}
	if _, err := os.Stat(featureFile); os.IsNotExist(err) {
		t.Fatalf("expected feature.txt to be restored on feature-1")
	}

	// 10. Test dirty tree protection: create untracked collision and attempt switch
	// Switch back to main first
	_, _ = executeCommand("checkout", "main")
	// Now recreate untracked feature.txt
	if err := os.WriteFile(featureFile, []byte("uncommitted conflicting edit"), 0644); err != nil {
		t.Fatalf("writing conflicting feature.txt: %v", err)
	}

	// Switching to feature-1 should FAIL to prevent clobbering untracked file
	_, err = executeCommand("checkout", "feature-1")
	if err == nil {
		t.Fatalf("expected checkout to fail due to dirty working tree protection, but it succeeded")
	}

	// Clean up untracked file to verify clean state
	_ = os.Remove(featureFile)

	// 11. Delete branch - cannot delete current branch
	_, _ = executeCommand("checkout", "main")
	_, err = executeCommand("branch", "-d", "main")
	if err == nil {
		t.Fatalf("expected error deleting current branch main")
	}

	// Delete feature-1 from main
	delOut, err := executeCommand("branch", "-d", "feature-1")
	if err != nil {
		t.Fatalf("delete feature-1 failed: %v", err)
	}
	if !strings.Contains(delOut, "Deleted branch feature-1") {
		t.Errorf("unexpected delete output: %s", delOut)
	}
}
