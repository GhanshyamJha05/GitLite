package integration_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGitLiteIgnoreWorkflow(t *testing.T) {
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

	// 1. Initialize repository
	if _, err := executeCommand("init"); err != nil {
		t.Fatalf("init failed: %v", err)
	}

	// 2. Create .gitliteignore ignoring *.log and secret/
	ignoreContent := "*.log\nsecret/\n!allowed.log\n"
	if err := os.WriteFile(filepath.Join(tempDir, ".gitliteignore"), []byte(ignoreContent), 0644); err != nil {
		t.Fatal(err)
	}

	// 3. Create ignored and non-ignored files
	if err := os.WriteFile(filepath.Join(tempDir, "app.log"), []byte("log data"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tempDir, "allowed.log"), []byte("special log"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tempDir, "main.go"), []byte("package main"), 0644); err != nil {
		t.Fatal(err)
	}

	secretDir := filepath.Join(tempDir, "secret")
	if err := os.MkdirAll(secretDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(secretDir, "key.pem"), []byte("secret key"), 0644); err != nil {
		t.Fatal(err)
	}

	// 4. Verify status: app.log and secret/ should NOT appear in untracked files
	statusOut, err := executeCommand("status")
	if err != nil {
		t.Fatalf("status failed: %v", err)
	}

	if strings.Contains(statusOut, "app.log") {
		t.Errorf("status should not list app.log as untracked, got: %s", statusOut)
	}
	if strings.Contains(statusOut, "secret") || strings.Contains(statusOut, "key.pem") {
		t.Errorf("status should not list secret/ as untracked, got: %s", statusOut)
	}
	if !strings.Contains(statusOut, "allowed.log") {
		t.Errorf("status should list negated allowed.log as untracked, got: %s", statusOut)
	}
	if !strings.Contains(statusOut, "main.go") {
		t.Errorf("status should list main.go as untracked, got: %s", statusOut)
	}

	// 5. Run add . and check that ignored files are NOT staged
	if _, err := executeCommand("add", "."); err != nil {
		t.Fatalf("add . failed: %v", err)
	}

	postAddStatus, err := executeCommand("status")
	if err != nil {
		t.Fatalf("status failed: %v", err)
	}

	if strings.Contains(postAddStatus, "new file:   app.log") {
		t.Errorf("app.log was staged despite .gitliteignore rule")
	}
	if strings.Contains(postAddStatus, "new file:   secret/key.pem") {
		t.Errorf("secret/key.pem was staged despite .gitliteignore rule")
	}
	if !strings.Contains(postAddStatus, "new file:   allowed.log") {
		t.Errorf("allowed.log should be staged due to negation rule")
	}
	if !strings.Contains(postAddStatus, "new file:   main.go") {
		t.Errorf("main.go should be staged")
	}
}
