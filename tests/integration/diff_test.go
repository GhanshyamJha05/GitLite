package integration_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiffEngineWorkflow(t *testing.T) {
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

	// 2. Create file and stage it
	fileA := filepath.Join(tempDir, "sample.txt")
	initialContent := "line 1: initial\nline 2: baseline\n"
	if err := os.WriteFile(fileA, []byte(initialContent), 0644); err != nil {
		t.Fatalf("writing sample.txt: %v", err)
	}
	_, _ = executeCommand("add", "sample.txt")

	// Test diff --staged against empty HEAD
	stagedDiffOut, err := executeCommand("diff", "--staged")
	if err != nil {
		t.Fatalf("diff --staged failed: %v", err)
	}
	if !strings.Contains(stagedDiffOut, "+++ b/sample.txt") || !strings.Contains(stagedDiffOut, "+line 1: initial") {
		t.Errorf("expected staged diff output, got: %s", stagedDiffOut)
	}

	// 3. Commit initial version
	_, err = executeCommand("commit", "-m", "initial commit")
	if err != nil {
		t.Fatalf("commit failed: %v", err)
	}

	// diff --staged should now be empty because index == HEAD
	cleanStagedDiff, err := executeCommand("diff", "--staged")
	if err != nil || cleanStagedDiff != "" {
		t.Errorf("expected empty diff --staged after commit, got: %s", cleanStagedDiff)
	}

	// 4. Modify working tree file without staging
	modifiedContent := "line 1: initial\nline 2: modified in working tree\nline 3: newly appended\n"
	if err := os.WriteFile(fileA, []byte(modifiedContent), 0644); err != nil {
		t.Fatalf("modifying sample.txt: %v", err)
	}

	// Test gitlite diff (working tree vs index)
	workDiffOut, err := executeCommand("diff")
	if err != nil {
		t.Fatalf("diff failed: %v", err)
	}
	if !strings.Contains(workDiffOut, "-line 2: baseline") || !strings.Contains(workDiffOut, "+line 2: modified in working tree") || !strings.Contains(workDiffOut, "+line 3: newly appended") {
		t.Errorf("unexpected worktree diff output:\n%s", workDiffOut)
	}

	// 5. Stage changes and commit as revision 2
	_, _ = executeCommand("add", "sample.txt")
	_, _ = executeCommand("commit", "-m", "second revision")

	// Get commit hashes from log
	logOut, _ := executeCommand("log")
	lines := strings.Split(logOut, "\n")
	var commitHashes []string
	for _, l := range lines {
		if strings.HasPrefix(l, "commit ") {
			commitHashes = append(commitHashes, strings.TrimPrefix(l, "commit "))
		}
	}

	if len(commitHashes) < 2 {
		t.Fatalf("expected at least 2 commits, found %d", len(commitHashes))
	}

	rev2 := commitHashes[0]
	rev1 := commitHashes[1]

	// 6. Test gitlite diff <rev1> <rev2>
	commitDiffOut, err := executeCommand("diff", rev1, rev2)
	if err != nil {
		t.Fatalf("diff rev1 rev2 failed: %v", err)
	}
	if !strings.Contains(commitDiffOut, "-line 2: baseline") || !strings.Contains(commitDiffOut, "+line 2: modified in working tree") {
		t.Errorf("unexpected commit-to-commit diff output:\n%s", commitDiffOut)
	}
}
