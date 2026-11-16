package integration_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCommitAndLogWorkflow(t *testing.T) {
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

	// 2. Commit on empty index must fail safely
	_, err = executeCommand("commit", "-m", "empty commit")
	if err == nil {
		t.Fatalf("expected commit to fail on empty index")
	}

	// 3. Stage a file
	filePath := filepath.Join(tempDir, "file1.txt")
	if err := os.WriteFile(filePath, []byte("version 1"), 0644); err != nil {
		t.Fatalf("writing file1: %v", err)
	}

	_, err = executeCommand("add", "file1.txt")
	if err != nil {
		t.Fatalf("add failed: %v", err)
	}

	// 4. Create first commit
	commitOut1, err := executeCommand("commit", "-m", "feat: first snapshot")
	if err != nil {
		t.Fatalf("first commit failed: %v", err)
	}
	if !strings.Contains(commitOut1, "[main ") || !strings.Contains(commitOut1, "feat: first snapshot") {
		t.Errorf("unexpected commit output: %s", commitOut1)
	}

	// 5. Stage modification and create second commit
	if err := os.WriteFile(filePath, []byte("version 2 modified"), 0644); err != nil {
		t.Fatalf("modifying file1: %v", err)
	}
	_, err = executeCommand("add", "file1.txt")
	if err != nil {
		t.Fatalf("second add failed: %v", err)
	}

	commitOut2, err := executeCommand("commit", "-m", "fix: updated file1 to version 2")
	if err != nil {
		t.Fatalf("second commit failed: %v", err)
	}
	if !strings.Contains(commitOut2, "fix: updated file1 to version 2") {
		t.Errorf("unexpected second commit output: %s", commitOut2)
	}

	// 6. Test gitlite log
	logOut, err := executeCommand("log")
	if err != nil {
		t.Fatalf("log failed: %v", err)
	}

	if !strings.Contains(logOut, "feat: first snapshot") || !strings.Contains(logOut, "fix: updated file1 to version 2") {
		t.Errorf("expected both commits in log output, got:\n%s", logOut)
	}

	// 7. Test gitlite show
	showOut, err := executeCommand("show")
	if err != nil {
		t.Fatalf("show failed: %v", err)
	}
	if !strings.Contains(showOut, "fix: updated file1 to version 2") || !strings.Contains(showOut, "tree   ") {
		t.Errorf("unexpected show output:\n%s", showOut)
	}
}

func TestCommitAmendAndAllFlags(t *testing.T) {
	tempDir := t.TempDir()

	_, err := executeCommand("init", tempDir)
	if err != nil {
		t.Fatalf("init failed: %v", err)
	}

	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("chdir failed: %v", err)
	}

	filePath := filepath.Join(tempDir, "auto.txt")
	if err := os.WriteFile(filePath, []byte("v1"), 0644); err != nil {
		t.Fatalf("writing auto.txt: %v", err)
	}
	if _, err := executeCommand("add", "auto.txt"); err != nil {
		t.Fatalf("add failed: %v", err)
	}
	if _, err := executeCommand("commit", "-m", "first commit"); err != nil {
		t.Fatalf("commit failed: %v", err)
	}

	// Modify auto.txt without running 'gitlite add'
	if err := os.WriteFile(filePath, []byte("v2 modified automatically"), 0644); err != nil {
		t.Fatalf("modifying auto.txt: %v", err)
	}

	// Commit with -a
	commitOut, err := executeCommand("commit", "-a", "-m", "commit with -a flag")
	if err != nil {
		t.Fatalf("commit -a failed: %v, out: %s", err, commitOut)
	}
	if !strings.Contains(commitOut, "commit with -a flag") {
		t.Errorf("expected commit output to contain message, got: %s", commitOut)
	}

	// Commit with --amend
	amendOut, err := executeCommand("commit", "--amend", "-m", "commit amended message")
	if err != nil {
		t.Fatalf("commit --amend failed: %v, out: %s", err, amendOut)
	}
	if !strings.Contains(amendOut, "commit amended message") {
		t.Errorf("expected amend output to contain amended message, got: %s", amendOut)
	}

	// Check log
	logOut, err := executeCommand("log")
	if err != nil {
		t.Fatalf("log failed: %v", err)
	}
	if !strings.Contains(logOut, "commit amended message") {
		t.Errorf("expected log to contain amended message, got: %s", logOut)
	}
	if strings.Contains(logOut, "commit with -a flag") {
		t.Errorf("amended message should replace original message in history")
	}
}
