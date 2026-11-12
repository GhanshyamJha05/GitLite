package webui

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
	"gitlite/internal/repository"
)

func TestWebUIHandlers(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "gitlite-webui-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	repo, err := repository.Init(tempDir)
	if err != nil {
		t.Fatalf("failed to init repo: %v", err)
	}

	sig := object.Signature{Name: "Tester", Email: "test@example.com", When: time.Now()}

	// Create initial file & commit
	filePath := filepath.Join(tempDir, "hello.txt")
	if err := os.WriteFile(filePath, []byte("Hello WebUI\n"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	if err := repo.Add(filePath); err != nil {
		t.Fatalf("failed to add file: %v", err)
	}

	c1, err := repo.Commit("feat: initial commit", sig, sig)
	if err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	// Create branch
	if err := repo.Refs().CreateBranch("feature", c1); err != nil {
		t.Fatalf("failed to create branch: %v", err)
	}

	server := NewServer(repo, 8080)
	handler := server.SetupHandler()

	// 1. Test index page
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for index, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "GitLite Visualizer") {
		t.Errorf("expected index html to contain title")
	}

	// 2. Test /api/graph
	req = httptest.NewRequest("GET", "/api/graph", nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for /api/graph, got %d", w.Code)
	}

	var graphResp GraphResponse
	if err := json.Unmarshal(w.Body.Bytes(), &graphResp); err != nil {
		t.Fatalf("failed to decode graph JSON: %v", err)
	}

	if len(graphResp.Commits) != 1 {
		t.Fatalf("expected 1 commit in graph, got %d", len(graphResp.Commits))
	}
	if graphResp.Commits[0].Hash != string(c1) {
		t.Errorf("expected commit hash %s, got %s", c1, graphResp.Commits[0].Hash)
	}

	// 3. Test /api/commit
	req = httptest.NewRequest("GET", "/api/commit?hash="+string(c1), nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for /api/commit, got %d", w.Code)
	}

	var commitDetail CommitDetailResponse
	if err := json.Unmarshal(w.Body.Bytes(), &commitDetail); err != nil {
		t.Fatalf("failed to decode commit detail: %v", err)
	}
	if commitDetail.Hash != string(c1) {
		t.Errorf("expected hash %s, got %s", c1, commitDetail.Hash)
	}
	if len(commitDetail.Diffs) != 1 {
		t.Errorf("expected 1 diff entry, got %d", len(commitDetail.Diffs))
	}

	// 4. Test /api/checkout
	body := strings.NewReader(`{"target":"feature"}`)
	req = httptest.NewRequest("POST", "/api/checkout", body)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for checkout, got %d: %s", w.Code, w.Body.String())
	}

	_, curBranch, _ := repo.Refs().ResolveHead()
	if curBranch != "feature" {
		t.Errorf("expected current branch to be feature, got %s", curBranch)
	}
}
