package integration_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGitLiteUndoWorkflow(t *testing.T) {
	tempDir := t.TempDir()
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current working directory: %v", err)
	}
	defer func() {
		_ = os.Chdir(originalWd)
	}()

	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("failed to change directory: %v", err)
	}

	// 1. Initialize repo
	if _, err := executeCommand("init"); err != nil {
		t.Fatalf("init failed: %v", err)
	}

	// 2. Undo with no history should report gracefully
	out, err := executeCommand("undo")
	if err != nil {
		t.Fatalf("expected undo on empty repo to succeed gracefully, got: %v", err)
	}
	if !strings.Contains(out, "No operations available to undo") {
		t.Errorf("expected no undo available notice, got: %s", out)
	}

	// 3. Create initial commit
	if err := os.WriteFile(filepath.Join(tempDir, "file1.txt"), []byte("version 1"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := executeCommand("add", "file1.txt"); err != nil {
		t.Fatal(err)
	}
	if _, err := executeCommand("commit", "-m", "first commit"); err != nil {
		t.Fatal(err)
	}

	// Verify log has 1 commit
	logOut, _ := executeCommand("log")
	if !strings.Contains(logOut, "first commit") {
		t.Fatalf("expected commit in log, got: %s", logOut)
	}

	// 4. Create second commit
	if err := os.WriteFile(filepath.Join(tempDir, "file2.txt"), []byte("version 2"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := executeCommand("add", "file2.txt"); err != nil {
		t.Fatal(err)
	}
	if _, err := executeCommand("commit", "-m", "second commit"); err != nil {
		t.Fatal(err)
	}

	logOut2, _ := executeCommand("log")
	if !strings.Contains(logOut2, "second commit") {
		t.Fatalf("expected second commit in log, got: %s", logOut2)
	}

	// 5. Run gitlite undo to undo second commit
	undoOut, err := executeCommand("undo")
	if err != nil {
		t.Fatalf("undo failed: %v", err)
	}
	if !strings.Contains(undoOut, "Undid operation: commit \"second commit\"") {
		t.Errorf("unexpected undo output: %s", undoOut)
	}

	// Verify log only has first commit now
	logOutAfterUndo, _ := executeCommand("log")
	if strings.Contains(logOutAfterUndo, "second commit") {
		t.Errorf("second commit should have been removed from history, got: %s", logOutAfterUndo)
	}
	if !strings.Contains(logOutAfterUndo, "first commit") {
		t.Errorf("first commit should still be present, got: %s", logOutAfterUndo)
	}

	// 6. Undo first commit (back to state before first commit, where file1.txt was staged)
	undoOut2, err := executeCommand("undo")
	if err != nil {
		t.Fatalf("undo 2 failed: %v", err)
	}
	if !strings.Contains(undoOut2, "Undid operation: commit \"first commit\"") {
		t.Errorf("unexpected undo 2 output: %s", undoOut2)
	}

	// Verify log is now empty (unborn branch notice)
	logOutEmpty, _ := executeCommand("log")
	if !strings.Contains(logOutEmpty, "does not have any commits yet") {
		t.Errorf("expected no commits message in log, got: %s", logOutEmpty)
	}

	// Status should show file1.txt staged (just like before commit 1 was run) and file2.txt untracked
	statusOut, _ := executeCommand("status")
	if !strings.Contains(statusOut, "new file:   file1.txt") {
		t.Errorf("expected file1.txt staged, got: %s", statusOut)
	}
	if !strings.Contains(statusOut, "file2.txt") {
		t.Errorf("expected file2.txt untracked, got: %s", statusOut)
	}
}
