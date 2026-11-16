package remote

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"gitlite/internal/object"
	"gitlite/internal/repository"
)

// FetchResult summarizes the commits fetched from a remote repository.
type FetchResult struct {
	RemoteName    string            `json:"remote_name"`
	RemoteURL     string            `json:"remote_url"`
	FetchedCommits int              `json:"fetched_commits"`
	UpdatedRefs   map[string]string `json:"updated_refs"` // branch -> commit hash
}

// Fetch fetches branches and objects from the remote repository without merging into local branches.
// Remote tracking branches are saved under refs/remotes/<remoteName>/<branch>.
func Fetch(repo *repository.Repository, remoteName string, tokenOverride string) (*FetchResult, error) {
	rem, err := repo.GetRemote(remoteName)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrRemoteNotConfigured, remoteName)
	}

	owner, repoName, err := ParseGitHubURL(rem.URL)
	if err != nil {
		return nil, err
	}

	token := tokenOverride
	if token == "" {
		token = rem.Token
	}
	if token == "" {
		token = os.Getenv("GITHUB_TOKEN")
	}
	if token == "" {
		token = os.Getenv("GH_TOKEN")
	}

	client := NewGitHubClient(owner, repoName, token)
	client.BaseURL = DefaultBaseURL

	return FetchWithClient(repo, client, remoteName)
}

// FetchWithClient downloads commits and objects using the provided GitHubClient.
func FetchWithClient(repo *repository.Repository, client *GitHubClient, remoteName string) (*FetchResult, error) {
	// 1. List remote branch references: GET /repos/{owner}/{repo}/git/matching-refs/heads
	resp, err := client.doRequest(http.MethodGet, "/git/matching-refs/heads", nil)
	if err != nil {
		return nil, fmt.Errorf("listing remote branches: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed fetching remote branches (status %d): %s", resp.StatusCode, string(bBytes))
	}

	type gitRef struct {
		Ref    string `json:"ref"`
		Object struct {
			Sha  string `json:"sha"`
			Type string `json:"type"`
		} `json:"object"`
	}

	var remoteRefs []gitRef
	if err := json.NewDecoder(resp.Body).Decode(&remoteRefs); err != nil {
		return nil, fmt.Errorf("decoding remote refs: %w", err)
	}

	updatedRefs := make(map[string]string)
	fetchedCommits := 0

	// Helper to fetch blob
	fetchBlob := func(blobSha string) error {
		h := object.Hash(blobSha)
		if repo.Store.Has(h) {
			return nil
		}

		bResp, err := client.doRequest(http.MethodGet, fmt.Sprintf("/git/blobs/%s", blobSha), nil)
		if err != nil {
			return err
		}
		defer bResp.Body.Close()

		if bResp.StatusCode != http.StatusOK {
			return fmt.Errorf("fetching blob %s: HTTP %d", blobSha, bResp.StatusCode)
		}

		type blobResponse struct {
			Content  string `json:"content"`
			Encoding string `json:"encoding"`
		}
		var br blobResponse
		if err := json.NewDecoder(bResp.Body).Decode(&br); err != nil {
			return err
		}

		var rawData []byte
		if br.Encoding == "base64" {
			cleanContent := strings.ReplaceAll(br.Content, "\n", "")
			cleanContent = strings.ReplaceAll(cleanContent, "\r", "")
			dec, err := base64.StdEncoding.DecodeString(cleanContent)
			if err != nil {
				return fmt.Errorf("decoding blob base64: %w", err)
			}
			rawData = dec
		} else {
			rawData = []byte(br.Content)
		}

		blobObj := object.NewBlob(rawData)
		_, err = repo.Store.Write(blobObj)
		return err
	}

	// Helper to fetch tree recursively
	var fetchTree func(treeSha string) error
	fetchTree = func(treeSha string) error {
		h := object.Hash(treeSha)
		if repo.Store.Has(h) {
			return nil
		}

		tResp, err := client.doRequest(http.MethodGet, fmt.Sprintf("/git/trees/%s?recursive=1", treeSha), nil)
		if err != nil {
			return err
		}
		defer tResp.Body.Close()

		if tResp.StatusCode != http.StatusOK {
			return fmt.Errorf("fetching tree %s: HTTP %d", treeSha, tResp.StatusCode)
		}

		type treeEntryItem struct {
			Path string `json:"path"`
			Mode string `json:"mode"`
			Type string `json:"type"`
			Sha  string `json:"sha"`
		}
		type treeResponse struct {
			Sha  string          `json:"sha"`
			Tree []treeEntryItem `json:"tree"`
		}

		var tr treeResponse
		if err := json.NewDecoder(tResp.Body).Decode(&tr); err != nil {
			return err
		}

		var entries []object.TreeEntry
		for _, item := range tr.Tree {
			if item.Type == "blob" {
				if err := fetchBlob(item.Sha); err != nil {
					return err
				}
				entries = append(entries, object.TreeEntry{
					Mode: object.FileMode(item.Mode),
					Name: filepath.Base(item.Path),
					Hash: object.Hash(item.Sha),
				})
			}
		}

		treeObj := object.NewTree(entries)
		_, err = repo.Store.Write(treeObj)
		return err
	}

	// Helper to fetch commit recursively
	var fetchCommit func(commitSha string) error
	fetchCommit = func(commitSha string) error {
		cHash := object.Hash(commitSha)
		if repo.Store.Has(cHash) {
			return nil
		}

		cResp, err := client.doRequest(http.MethodGet, fmt.Sprintf("/git/commits/%s", commitSha), nil)
		if err != nil {
			return err
		}
		defer cResp.Body.Close()

		if cResp.StatusCode != http.StatusOK {
			return fmt.Errorf("fetching commit %s: HTTP %d", commitSha, cResp.StatusCode)
		}

		type commitResponse struct {
			Sha     string `json:"sha"`
			Message string `json:"message"`
			Tree    struct {
				Sha string `json:"sha"`
			} `json:"tree"`
			Parents []struct {
				Sha string `json:"sha"`
			} `json:"parents"`
			Author struct {
				Name  string `json:"name"`
				Email string `json:"email"`
				Date  string `json:"date"`
			} `json:"author"`
			Committer struct {
				Name  string `json:"name"`
				Email string `json:"email"`
				Date  string `json:"date"`
			} `json:"committer"`
		}

		var cr commitResponse
		if err := json.NewDecoder(cResp.Body).Decode(&cr); err != nil {
			return err
		}

		// Fetch tree
		if err := fetchTree(cr.Tree.Sha); err != nil {
			return err
		}

		var parents []object.Hash
		for _, p := range cr.Parents {
			parents = append(parents, object.Hash(p.Sha))
			if err := fetchCommit(p.Sha); err != nil {
				return err
			}
		}

		authorSig := object.Signature{Name: cr.Author.Name, Email: cr.Author.Email}
		committerSig := object.Signature{Name: cr.Committer.Name, Email: cr.Committer.Email}

		cObj := object.NewCommit(object.Hash(cr.Tree.Sha), parents, authorSig, committerSig, cr.Message)
		if _, err := repo.Store.Write(cObj); err != nil {
			return err
		}
		fetchedCommits++
		return nil
	}

	// 2. Fetch all commits for each matching branch and store refs/remotes/<remoteName>/<branch>
	for _, ref := range remoteRefs {
		branchName := strings.TrimPrefix(ref.Ref, "refs/heads/")
		targetSha := ref.Object.Sha

		if err := fetchCommit(targetSha); err != nil {
			return nil, fmt.Errorf("fetching history for branch %s: %w", branchName, err)
		}

		// Store remote tracking ref: refs/remotes/<remoteName>/<branch>
		remoteRefDir := filepath.Join(repo.GitLiteDir, "refs", "remotes", remoteName)
		_ = os.MkdirAll(remoteRefDir, 0755)
		refFile := filepath.Join(remoteRefDir, branchName)
		_ = os.WriteFile(refFile, []byte(targetSha+"\n"), 0644)

		updatedRefs[branchName] = targetSha
	}

	return &FetchResult{
		RemoteName:    remoteName,
		RemoteURL:     fmt.Sprintf("https://github.com/%s/%s", client.Owner, client.Repo),
		FetchedCommits: fetchedCommits,
		UpdatedRefs:   updatedRefs,
	}, nil
}
