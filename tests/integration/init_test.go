package integration_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"gitlite/internal/cli"
	"gitlite/internal/repository"
)

func executeCommand(args ...string) (string, error) {
	cmd := cli.GetRootCmd()
	buf := new(bytes.Buffer)

	var setOutRecursive func(c *cobra.Command)
	setOutRecursive = func(c *cobra.Command) {
		c.SetOut(buf)
		c.SetErr(buf)
		for _, sub := range c.Commands() {
			setOutRecursive(sub)
		}
	}
	setOutRecursive(cmd)

	cmd.SetArgs(args)
	err := cmd.Execute()
	out := buf.String()
	return out, err
}

func TestCLIInit(t *testing.T) {
	tempDir := t.TempDir()
	targetRepo := filepath.Join(tempDir, "myrepo")

	// Test gitlite init <path>
	output, err := executeCommand("init", targetRepo)
	if err != nil {
		t.Fatalf("gitlite init failed: %v", err)
	}

	expectedSub := "Initialized empty GitLite repository"
	if !strings.Contains(output, expectedSub) {
		t.Errorf("expected output to contain %q, got: %s", expectedSub, output)
	}

	// Verify .gitlite directory exists
	gitliteDir := filepath.Join(targetRepo, repository.DotGitLite)
	if _, err := os.Stat(gitliteDir); os.IsNotExist(err) {
		t.Fatalf("expected %s to exist", gitliteDir)
	}

	// Second init on same directory should fail (idempotency check)
	_, err = executeCommand("init", targetRepo)
	if err == nil {
		t.Fatalf("expected error when reinitializing, got nil")
	}
}

