package integration

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gitlite/internal/object"
	"gitlite/internal/repository"
	"gitlite/internal/webui"
)

func TestGitLiteUIWorkflow(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "gitlite-ui-integration-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	repo, err := repository.Init(tempDir)
	if err != nil {
		t.Fatalf("failed to init repository: %v", err)
	}

	sig := object.Signature{
		Name:  "Integration Tester",
		Email: "tester@gitlite.local",
		When:  time.Now(),
	}

	// 1. First commit
	fileA := filepath.Join(tempDir, "file_a.txt")
	if err := os.WriteFile(fileA, []byte("line 1\nline 2\n"), 0644); err != nil {
		t.Fatalf("failed to write fileA: %v", err)
	}
	if err := repo.Add(fileA); err != nil {
		t.Fatalf("failed to add fileA: %v", err)
	}
	c1, err := repo.Commit("commit one: add fileA", sig, sig)
	if err != nil {
		t.Fatalf("failed to commit c1: %v", err)
	}

	// 2. Second commit on feature branch
	if err := repo.Refs().CreateBranch("dev", c1); err != nil {
		t.Fatalf("failed to create branch dev: %v", err)
	}
	if err := repo.Checkout("dev"); err != nil {
		t.Fatalf("failed to checkout dev: %v", err)
	}

	fileB := filepath.Join(tempDir, "file_b.txt")
	if err := os.WriteFile(fileB, []byte("feature branch content\n"), 0644); err != nil {
		t.Fatalf("failed to write fileB: %v", err)
	}
	if err := repo.Add(fileB); err != nil {
		t.Fatalf("failed to add fileB: %v", err)
	}
	c2, err := repo.Commit("commit two: add fileB on dev", sig, sig)
	if err != nil {
		t.Fatalf("failed to commit c2: %v", err)
	}

	// Start server on free port
	server := webui.NewServer(repo, 0)
	readyChan := make(chan string, 1)

	go func() {
		_ = server.Start(readyChan)
	}()

	var baseURL string
	select {
	case baseURL = <-readyChan:
	case <-time.After(5 * time.Second):
		t.Fatalf("web server did not become ready within 5 seconds")
	}

	// 3. Test HTTP GET /
	resp, err := http.Get(baseURL + "/")
	if err != nil {
		t.Fatalf("failed to GET /: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 for /, got %d", resp.StatusCode)
	}

	// 4. Test HTTP GET /api/graph
	resp, err = http.Get(baseURL + "/api/graph")
	if err != nil {
		t.Fatalf("failed to GET /api/graph: %v", err)
	}
	var graphResp webui.GraphResponse
	if err := json.NewDecoder(resp.Body).Decode(&graphResp); err != nil {
		_ = resp.Body.Close()
		t.Fatalf("failed to decode /api/graph JSON: %v", err)
	}
	_ = resp.Body.Close()

	if len(graphResp.Commits) != 2 {
		t.Fatalf("expected 2 commits in graph, got %d", len(graphResp.Commits))
	}
	if graphResp.CurrentBranch != "dev" {
		t.Errorf("expected current branch 'dev', got %s", graphResp.CurrentBranch)
	}

	// 5. Test HTTP GET /api/commit?hash=<c2>
	resp, err = http.Get(baseURL + "/api/commit?hash=" + string(c2))
	if err != nil {
		t.Fatalf("failed to GET /api/commit: %v", err)
	}
	var detail webui.CommitDetailResponse
	if err := json.NewDecoder(resp.Body).Decode(&detail); err != nil {
		_ = resp.Body.Close()
		t.Fatalf("failed to decode commit detail: %v", err)
	}
	_ = resp.Body.Close()

	if detail.Hash != string(c2) {
		t.Errorf("expected commit hash %s, got %s", c2, detail.Hash)
	}
	if len(detail.Diffs) != 1 || detail.Diffs[0].Path != "file_b.txt" {
		t.Errorf("expected 1 diff on file_b.txt, got %+v", detail.Diffs)
	}
}
