package repository_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gitlite/internal/repository"
)

func TestInitSuccess(t *testing.T) {
	tempDir := t.TempDir()

	repo, err := repository.Init(tempDir)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	if repo.WorkTree != tempDir {
		t.Errorf("expected WorkTree %s, got %s", tempDir, repo.WorkTree)
	}

	gitliteDir := filepath.Join(tempDir, repository.DotGitLite)
	if repo.GitLiteDir != gitliteDir {
		t.Errorf("expected GitLiteDir %s, got %s", gitliteDir, repo.GitLiteDir)
	}

	// Verify required subdirectories
	requiredDirs := []string{
		filepath.Join(gitliteDir, repository.ObjectsDir),
		filepath.Join(gitliteDir, filepath.FromSlash(repository.HeadsDir)),
	}

	for _, dir := range requiredDirs {
		info, err := os.Stat(dir)
		if err != nil {
			t.Errorf("expected directory %s to exist: %v", dir, err)
		} else if !info.IsDir() {
			t.Errorf("expected %s to be a directory", dir)
		}
	}

	// Verify HEAD contents
	headPath := filepath.Join(gitliteDir, repository.HeadFile)
	headBytes, err := os.ReadFile(headPath)
	if err != nil {
		t.Fatalf("reading HEAD file: %v", err)
	}
	expectedHead := "ref: refs/heads/main\n"
	if string(headBytes) != expectedHead {
		t.Errorf("expected HEAD %q, got %q", expectedHead, string(headBytes))
	}

	// Verify config file exists
	configPath := filepath.Join(gitliteDir, repository.ConfigFile)
	if _, err := os.Stat(configPath); err != nil {
		t.Errorf("expected config file to exist: %v", err)
	}
}

func TestInitAlreadyExists(t *testing.T) {
	tempDir := t.TempDir()

	_, err := repository.Init(tempDir)
	if err != nil {
		t.Fatalf("first Init failed: %v", err)
	}

	// Re-initializing in the same directory must fail
	_, err = repository.Init(tempDir)
	if err == nil {
		t.Fatalf("expected error on re-initialization, got nil")
	}

	if !strings.Contains(err.Error(), repository.ErrAlreadyInitialized.Error()) {
		t.Errorf("expected ErrAlreadyInitialized, got: %v", err)
	}
}

func TestDiscover(t *testing.T) {
	tempDir := t.TempDir()

	// Before init, discover must return ErrRepositoryNotFound
	_, err := repository.Discover(tempDir)
	if err != repository.ErrRepositoryNotFound {
		t.Fatalf("expected ErrRepositoryNotFound, got: %v", err)
	}

	// Initialize repo
	repo, err := repository.Init(tempDir)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Discover from root
	discovered, err := repository.Discover(tempDir)
	if err != nil {
		t.Fatalf("Discover from root failed: %v", err)
	}
	if discovered.WorkTree != repo.WorkTree {
		t.Errorf("expected %s, got %s", repo.WorkTree, discovered.WorkTree)
	}

	// Discover from deep subdirectory
	subDir := filepath.Join(tempDir, "a", "b", "c")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("creating subdir: %v", err)
	}

	discoveredSub, err := repository.Discover(subDir)
	if err != nil {
		t.Fatalf("Discover from subDir failed: %v", err)
	}
	if discoveredSub.WorkTree != repo.WorkTree {
		t.Errorf("expected %s, got %s", repo.WorkTree, discoveredSub.WorkTree)
	}
}

