package remote

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gitlite/internal/object"
	"gitlite/internal/repository"
)

func TestParseGitHubURL(t *testing.T) {
	tests := []struct {
		url       string
		wantOwner string
		wantRepo  string
		wantErr   bool
	}{
		{"https://github.com/octocat/Hello-World.git", "octocat", "Hello-World", false},
		{"https://github.com/octocat/Hello-World", "octocat", "Hello-World", false},
		{"git@github.com:octocat/Hello-World.git", "octocat", "Hello-World", false},
		{"git@github.com:octocat/Hello-World", "octocat", "Hello-World", false},
		{"invalid-url", "", "", true},
	}

	for _, tt := range tests {
		owner, repo, err := ParseGitHubURL(tt.url)
		if (err != nil) != tt.wantErr {
			t.Errorf("ParseGitHubURL(%q) err = %v, wantErr %v", tt.url, err, tt.wantErr)
			continue
		}
		if owner != tt.wantOwner || repo != tt.wantRepo {
			t.Errorf("ParseGitHubURL(%q) = (%q, %q), want (%q, %q)", tt.url, owner, repo, tt.wantOwner, tt.wantRepo)
		}
	}
}

func TestPushMockServer(t *testing.T) {
	blobsCount := 0
	treesCount := 0
	commitsCount := 0
	refUpdated := false

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/repos/testowner/testrepo/git/blobs":
			blobsCount++
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]string{"sha": "mock-blob-sha"})

		case r.Method == http.MethodPost && r.URL.Path == "/repos/testowner/testrepo/git/trees":
			treesCount++
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]string{"sha": "mock-tree-sha"})

		case r.Method == http.MethodPost && r.URL.Path == "/repos/testowner/testrepo/git/commits":
			commitsCount++
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]string{"sha": "mock-commit-sha"})

		case r.Method == http.MethodPatch && r.URL.Path == "/repos/testowner/testrepo/git/refs/heads/main":
			refUpdated = true
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]string{"ref": "refs/heads/main"})

		case r.Method == http.MethodPost && r.URL.Path == "/repos/testowner/testrepo/git/refs":
			refUpdated = true
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]string{"ref": "refs/heads/main"})

		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	// Create test repo and a commit
	tmpDir, err := os.MkdirTemp("", "gitlite-push-test-*")
	if err != nil {
		t.Fatalf("temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	repo, err := repository.Init(tmpDir)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	testFile := filepath.Join(tmpDir, "README.md")
	if err := os.WriteFile(testFile, []byte("Hello push!"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	if err := repo.Add(testFile); err != nil {
		t.Fatalf("stage file: %v", err)
	}

	sig := object.Signature{
		Name:  "Tester",
		Email: "test@gitlite.local",
		When:  time.Now(),
	}
	cHash, err := repo.Commit("feat: initial test commit", sig, sig)
	if err != nil {
		t.Fatalf("commit failed: %v", err)
	}
	if cHash == "" {
		t.Fatal("empty commit hash")
	}

	// Add remote pointing to mock
	mockURL := ts.URL + "/testowner/testrepo.git"
	if err := repo.AddRemote("origin", mockURL, "mock-token"); err != nil {
		t.Fatalf("add remote: %v", err)
	}

	client := NewGitHubClient("testowner", "testrepo", "mock-token")
	client.BaseURL = ts.URL
	client.HTTPClient = ts.Client()

	// Invoke mock push directly
	res, err := PushWithClient(repo, client, "main")
	if err != nil {
		t.Fatalf("PushWithClient failed: %v", err)
	}

	if res.PushedBlobs != 1 {
		t.Errorf("expected 1 blob pushed, got %d", res.PushedBlobs)
	}
	if res.PushedTrees != 1 {
		t.Errorf("expected 1 tree pushed, got %d", res.PushedTrees)
	}
	if res.PushedCommits != 1 {
		t.Errorf("expected 1 commit pushed, got %d", res.PushedCommits)
	}
	if !refUpdated {
		t.Errorf("expected ref to be updated on remote")
	}
}

func TestPushWithForceAndUpstream(t *testing.T) {
	forceReceived := false
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/repos/testowner/testrepo/git/blobs":
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]string{"sha": "mock-blob-sha"})
		case r.Method == http.MethodPost && r.URL.Path == "/repos/testowner/testrepo/git/trees":
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]string{"sha": "mock-tree-sha"})
		case r.Method == http.MethodPost && r.URL.Path == "/repos/testowner/testrepo/git/commits":
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]string{"sha": "mock-commit-sha"})
		case r.Method == http.MethodPatch && r.URL.Path == "/repos/testowner/testrepo/git/refs/heads/main":
			var body map[string]interface{}
			_ = json.NewDecoder(r.Body).Decode(&body)
			if f, ok := body["force"].(bool); ok && f {
				forceReceived = true
			}
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]string{"ref": "refs/heads/main"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	tmpDir, err := os.MkdirTemp("", "gitlite-push-force-*")
	if err != nil {
		t.Fatalf("temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	repo, err := repository.Init(tmpDir)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	testFile := filepath.Join(tmpDir, "file.txt")
	_ = os.WriteFile(testFile, []byte("Content"), 0644)
	_ = repo.Add(testFile)
	sig := object.Signature{Name: "Tester", Email: "test@example.com", When: time.Now()}
	_, _ = repo.Commit("commit", sig, sig)

	client := NewGitHubClient("testowner", "testrepo", "mock-token")
	client.BaseURL = ts.URL
	client.HTTPClient = ts.Client()

	res, err := PushWithClientOptions(repo, client, "main", true)
	if err != nil {
		t.Fatalf("PushWithClientOptions force failed: %v", err)
	}
	if !forceReceived {
		t.Errorf("expected force=true to be sent in payload")
	}
	if res.Branch != "main" {
		t.Errorf("expected branch main, got %s", res.Branch)
	}
}
