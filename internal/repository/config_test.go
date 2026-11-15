package repository

import (
	"os"
	"testing"
)

func TestConfigRemotes(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "gitlite-config-test-*")
	if err != nil {
		t.Fatalf("creating temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	repo, err := Init(tmpDir)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Initial load
	cfg, err := repo.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	if len(cfg.Remotes) != 0 {
		t.Errorf("expected 0 remotes, got %d", len(cfg.Remotes))
	}

	// Add remote
	err = repo.AddRemote("origin", "https://github.com/user/demo.git", "ghp_secret")
	if err != nil {
		t.Fatalf("AddRemote failed: %v", err)
	}

	// Get remote
	rem, err := repo.GetRemote("origin")
	if err != nil {
		t.Fatalf("GetRemote failed: %v", err)
	}
	if rem.Name != "origin" || rem.URL != "https://github.com/user/demo.git" || rem.Token != "ghp_secret" {
		t.Fatalf("unexpected remote data: %+v", rem)
	}

	// Remove remote
	err = repo.RemoveRemote("origin")
	if err != nil {
		t.Fatalf("RemoveRemote failed: %v", err)
	}

	_, err = repo.GetRemote("origin")
	if err == nil {
		t.Fatalf("expected error getting deleted remote, got nil")
	}
}
