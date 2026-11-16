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

	// 5. Test /api/remote/add
	remBody := strings.NewReader(`{"name":"origin","url":"https://github.com/testowner/testrepo.git","token":"secret"}`)
	req = httptest.NewRequest("POST", "/api/remote/add", remBody)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for remote add, got %d: %s", w.Code, w.Body.String())
	}

	rem, err := repo.GetRemote("origin")
	if err != nil {
		t.Fatalf("expected remote origin to exist: %v", err)
	}
	if rem.URL != "https://github.com/testowner/testrepo.git" {
		t.Errorf("unexpected remote url: %s", rem.URL)
	}

	// 6. Test /api/stage
	file2 := filepath.Join(tempDir, "file2.txt")
	_ = os.WriteFile(file2, []byte("Content of file 2"), 0644)
	stageBody := strings.NewReader(`{"path":"file2.txt"}`)
	req = httptest.NewRequest("POST", "/api/stage", stageBody)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for stage, got %d: %s", w.Code, w.Body.String())
	}

	// 7. Test /api/commit (POST)
	commitBody := strings.NewReader(`{"message":"feat: second commit from webui"}`)
	req = httptest.NewRequest("POST", "/api/commit", commitBody)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for commit POST, got %d: %s", w.Code, w.Body.String())
	}
	var commitResp struct {
		Success bool   `json:"success"`
		Hash    string `json:"hash"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &commitResp)
	if !commitResp.Success || commitResp.Hash == "" {
		t.Errorf("expected commit success with hash, got %+v", commitResp)
	}

	// 8. Test /api/branch (GET and POST)
	req = httptest.NewRequest("GET", "/api/branch", nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for branch GET, got %d", w.Code)
	}

	branchBody := strings.NewReader(`{"name":"new-web-branch"}`)
	req = httptest.NewRequest("POST", "/api/branch", branchBody)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for branch POST, got %d: %s", w.Code, w.Body.String())
	}

	// 9. Test /api/diff
	req = httptest.NewRequest("GET", "/api/diff?from="+string(c1)+"&to="+commitResp.Hash, nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for diff GET, got %d: %s", w.Code, w.Body.String())
	}
	var diffResp struct {
		Status string `json:"status"`
		Diff   string `json:"diff"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &diffResp)
	if diffResp.Status != "ok" || !strings.Contains(diffResp.Diff, "file2.txt") {
		t.Errorf("expected diff to contain file2.txt, got %+v", diffResp)
	}

	// 10. Test CSRF blocking external origins
	csrfBody := strings.NewReader(`{"name":"malicious-branch"}`)
	req = httptest.NewRequest("POST", "/api/branch", csrfBody)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://attacker-site.com")
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden for cross-site POST, got %d", w.Code)
	}
}
