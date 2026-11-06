package integration_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFsckWorkflow(t *testing.T) {
	tempDir := t.TempDir()

	_, _ = executeCommand("init", tempDir)
	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	_ = os.Chdir(tempDir)

	// Clean initial fsck
	fsckOut1, err := executeCommand("fsck")
	if err != nil {
		t.Fatalf("initial fsck failed: %v", err)
	}
	if !strings.Contains(fsckOut1, "Checking object directories") {
		t.Errorf("expected clean fsck output, got: %s", fsckOut1)
	}

	// Create commit
	fileA := filepath.Join(tempDir, "healthy.txt")
	_ = os.WriteFile(fileA, []byte("healthy file content\n"), 0644)
	_, _ = executeCommand("add", "healthy.txt")
	_, _ = executeCommand("commit", "-m", "healthy commit")

	// Verify fsck passes on healthy repo
	fsckOut2, err := executeCommand("fsck")
	if err != nil {
		t.Fatalf("healthy fsck failed: %v", err)
	}
	if !strings.Contains(fsckOut2, "100%") {
		t.Errorf("expected 100%% valid objects, got: %s", fsckOut2)
	}

	// Tamper with an object in .gitlite/objects to simulate bit rot / disk corruption
	objectsDir := filepath.Join(tempDir, ".gitlite", "objects")
	var corruptedPath string
	_ = filepath.Walk(objectsDir, func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() && corruptedPath == "" {
			corruptedPath = path
		}
		return nil
	})

	if corruptedPath == "" {
		t.Fatalf("failed to locate object file to corrupt")
	}

	// Corrupt the object payload
	_ = os.WriteFile(corruptedPath, []byte("tampered corrupted bytes!"), 0644)

	// fsck must detect the corruption
	fsckCorruptOut, err := executeCommand("fsck")
	if err != nil {
		t.Fatalf("fsck execution error: %v", err)
	}

	if !strings.Contains(fsckCorruptOut, "ERROR: corrupt object") {
		t.Errorf("expected fsck to detect corrupt object, got:\n%s", fsckCorruptOut)
	}
}
