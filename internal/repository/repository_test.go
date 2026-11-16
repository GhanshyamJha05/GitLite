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

func TestCommitAmend(t *testing.T) {
	tempDir := t.TempDir()
	repo, err := repository.Init(tempDir)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	sig := repository.DefaultSignature()

	// Amend on empty repo should fail
	_, err = repo.CommitAmend("cannot amend empty", sig, sig)
	if err == nil {
		t.Fatalf("expected error amending empty repository")
	}

	// First commit
	f1 := filepath.Join(tempDir, "f1.txt")
	_ = os.WriteFile(f1, []byte("f1 content"), 0644)
	if err := repo.Add(f1); err != nil {
		t.Fatalf("add f1: %v", err)
	}
	c1, err := repo.Commit("first commit", sig, sig)
	if err != nil {
		t.Fatalf("commit c1: %v", err)
	}

	// Amend first commit with updated message and additional file
	f2 := filepath.Join(tempDir, "f2.txt")
	_ = os.WriteFile(f2, []byte("f2 content"), 0644)
	if err := repo.Add(f2); err != nil {
		t.Fatalf("add f2: %v", err)
	}

	c1Amended, err := repo.CommitAmend("first commit amended", sig, sig)
	if err != nil {
		t.Fatalf("commit amend failed: %v", err)
	}
	if c1Amended == c1 {
		t.Fatalf("amended commit hash should be different from original commit")
	}

	head, _, err := repo.Refs().ResolveHead()
	if err != nil {
		t.Fatalf("resolve head: %v", err)
	}
	if head != c1Amended {
		t.Errorf("expected HEAD to point to amended commit %s, got %s", c1Amended, head)
	}

	commitObj, err := repo.Store.ReadCommit(c1Amended)
	if err != nil {
		t.Fatalf("read commit object: %v", err)
	}
	if commitObj.Message != "first commit amended" {
		t.Errorf("expected amended message 'first commit amended', got %q", commitObj.Message)
	}
	if len(commitObj.Parents) != 0 {
		t.Errorf("expected root amended commit to have 0 parents, got %d", len(commitObj.Parents))
	}

	// Add second commit
	f3 := filepath.Join(tempDir, "f3.txt")
	_ = os.WriteFile(f3, []byte("f3 content"), 0644)
	_ = repo.Add(f3)
	c2, err := repo.Commit("second commit", sig, sig)
	if err != nil {
		t.Fatalf("commit c2: %v", err)
	}

	// Amend second commit
	c2Amended, err := repo.CommitAmend("second commit amended", sig, sig)
	if err != nil {
		t.Fatalf("commit amend c2: %v", err)
	}

	c2Obj, err := repo.Store.ReadCommit(c2Amended)
	if err != nil {
		t.Fatalf("read commit c2 amended: %v", err)
	}
	if c2Obj.Message != "second commit amended" {
		t.Errorf("expected message 'second commit amended', got %q", c2Obj.Message)
	}
	if len(c2Obj.Parents) != 1 || c2Obj.Parents[0] != c1Amended {
		t.Errorf("expected parent of amended c2 to be c1Amended (%s), got %v", c1Amended, c2Obj.Parents)
	}
	if c2Amended == c2 {
		t.Errorf("c2 and c2Amended should have different hashes")
	}
}
