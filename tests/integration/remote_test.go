package integration

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gitlite/internal/object"
	"gitlite/internal/remote"
	"gitlite/internal/repository"
)

func TestRemoteAndPushWorkflow(t *testing.T) {
	// 1. Mock GitHub API server
	pushedBlobs := 0
	pushedTrees := 0
	pushedCommits := 0
	refPatched := false

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/git/blobs"):
			pushedBlobs++
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]string{"sha": "mock-blob-sha"})

		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/git/trees"):
			pushedTrees++
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]string{"sha": "mock-tree-sha"})

		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/git/commits"):
			pushedCommits++
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]string{"sha": "mock-commit-sha"})

		case r.Method == http.MethodPatch && strings.Contains(r.URL.Path, "/git/refs/heads/main"):
			refPatched = true
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]string{"ref": "refs/heads/main"})

		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/git/refs"):
			refPatched = true
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]string{"ref": "refs/heads/main"})

		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	// 2. Initialize test repo
	tempDir, err := os.MkdirTemp("", "gitlite-remote-int-test-*")
	if err != nil {
		t.Fatalf("temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	repo, err := repository.Init(tempDir)
	if err != nil {
		t.Fatalf("init repo: %v", err)
	}

	docFile := filepath.Join(tempDir, "architecture.md")
	if err := os.WriteFile(docFile, []byte("# GitLite Architecture\nIndependent VCS in Go.\n"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	if err := repo.Add(docFile); err != nil {
		t.Fatalf("add: %v", err)
	}

	sig := object.Signature{Name: "Ghanshyam Jha", Email: "gjha@example.com", When: time.Now()}
	cHash, err := repo.Commit("feat: add architecture documentation", sig, sig)
	if err != nil {
		t.Fatalf("commit: %v", err)
	}

	// 3. Add remote
	mockURL := ts.URL + "/GhanshyamJha05/GitLite.git"
	if err := repo.AddRemote("origin", mockURL, "test-pat-token"); err != nil {
		t.Fatalf("add remote: %v", err)
	}

	rem, err := repo.GetRemote("origin")
	if err != nil {
		t.Fatalf("get remote: %v", err)
	}
	if rem.URL != mockURL {
		t.Fatalf("expected remote url %s, got %s", mockURL, rem.URL)
	}

	// 4. Test Push with custom client pointed to test server
	client := remote.NewGitHubClient("GhanshyamJha05", "GitLite", "test-pat-token")
	client.BaseURL = ts.URL
	client.HTTPClient = ts.Client()

	pushRes, err := remote.PushWithClient(repo, client, "main")
	if err != nil {
		t.Fatalf("push failed: %v", err)
	}

	if pushRes.Branch != "main" {
		t.Errorf("expected branch main, got %s", pushRes.Branch)
	}
	if pushRes.HeadCommit != string(cHash) {
		t.Errorf("expected head commit %s, got %s", cHash, pushRes.HeadCommit)
	}
	if pushRes.PushedCommits != 1 {
		t.Errorf("expected 1 pushed commit, got %d", pushRes.PushedCommits)
	}
	if !refPatched {
		t.Errorf("expected remote ref to be updated")
	}
}
