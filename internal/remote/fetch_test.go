package remote

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"gitlite/internal/object"
	"gitlite/internal/repository"
)

func TestFetchPullCloneMockServer(t *testing.T) {
	// Prepare mock object hashes
	blobContent := "Hello from remote GitLite"
	blobObj := object.NewBlob([]byte(blobContent))
	blobSha := string(object.ComputeHash(object.EncodeCanonical(blobObj)))

	treeEntry := object.TreeEntry{
		Mode: object.ModeRegular,
		Name: "greeting.txt",
		Hash: object.Hash(blobSha),
	}
	treeObj := object.NewTree([]object.TreeEntry{treeEntry})
	treeSha := string(object.ComputeHash(object.EncodeCanonical(treeObj)))

	authorSig := object.Signature{Name: "RemoteDev", Email: "remote@example.com"}
	commitObj := object.NewCommit(object.Hash(treeSha), nil, authorSig, authorSig, "Remote initial commit")
	commitSha := string(object.ComputeHash(object.EncodeCanonical(commitObj)))

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/repos/testowner/testrepo/git/matching-refs/heads":
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode([]map[string]interface{}{
				{
					"ref": "refs/heads/main",
					"object": map[string]string{
						"sha":  commitSha,
						"type": "commit",
					},
				},
			})

		case r.Method == http.MethodGet && r.URL.Path == "/repos/testowner/testrepo/git/commits/"+commitSha:
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"sha":     commitSha,
				"message": "Remote initial commit",
				"tree": map[string]string{
					"sha": treeSha,
				},
				"parents": []interface{}{},
				"author": map[string]string{
					"name":  authorSig.Name,
					"email": authorSig.Email,
				},
				"committer": map[string]string{
					"name":  authorSig.Name,
					"email": authorSig.Email,
				},
			})

		case r.Method == http.MethodGet && r.URL.Path == "/repos/testowner/testrepo/git/trees/"+treeSha:
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"sha": treeSha,
				"tree": []map[string]string{
					{
						"path": "greeting.txt",
						"mode": "100644",
						"type": "blob",
						"sha":  blobSha,
					},
				},
			})

		case r.Method == http.MethodGet && r.URL.Path == "/repos/testowner/testrepo/git/blobs/"+blobSha:
			w.WriteHeader(http.StatusOK)
			encoded := base64.StdEncoding.EncodeToString([]byte(blobContent))
			_ = json.NewEncoder(w).Encode(map[string]string{
				"sha":      blobSha,
				"content":  encoded,
				"encoding": "base64",
			})

		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	// 1. Test FetchWithClient
	tmpDir, err := os.MkdirTemp("", "gitlite-fetch-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	repo, err := repository.Init(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	client := NewGitHubClient("testowner", "testrepo", "mock-token")
	client.BaseURL = ts.URL
	client.HTTPClient = ts.Client()

	fetchRes, err := FetchWithClient(repo, client, "origin")
	if err != nil {
		t.Fatalf("FetchWithClient failed: %v", err)
	}

	if fetchRes.FetchedCommits != 1 {
		t.Errorf("expected 1 fetched commit, got %d", fetchRes.FetchedCommits)
	}
	if fetchRes.UpdatedRefs["main"] != commitSha {
		t.Errorf("expected updated ref main = %s, got %s", commitSha, fetchRes.UpdatedRefs["main"])
	}

	// Verify remote tracking ref exists on disk
	remoteRefPath := filepath.Join(repo.GitLiteDir, "refs", "remotes", "origin", "main")
	if data, err := os.ReadFile(remoteRefPath); err != nil || string(data) != commitSha+"\n" {
		t.Errorf("remote ref file content mismatch: data=%q, err=%v", string(data), err)
	}

	// Verify fetched objects exist in CAS
	if !repo.Store.Has(object.Hash(commitSha)) {
		t.Errorf("expected commit %s to exist in store", commitSha)
	}
	if !repo.Store.Has(object.Hash(treeSha)) {
		t.Errorf("expected tree %s to exist in store", treeSha)
	}
	if !repo.Store.Has(object.Hash(blobSha)) {
		t.Errorf("expected blob %s to exist in store", blobSha)
	}

	// Verify reset to fetched commit populates the working directory
	if err := repo.Refs().UpdateHead(object.Hash(commitSha)); err != nil {
		t.Fatalf("update head: %v", err)
	}
	if err := repo.Reset(object.Hash(commitSha), repository.ResetHard); err != nil {
		t.Fatalf("reset hard: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(tmpDir, "greeting.txt"))
	if err != nil || string(content) != blobContent {
		t.Errorf("working tree file mismatch: content=%q, err=%v", string(content), err)
	}
}
