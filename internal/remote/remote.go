package remote

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"gitlite/internal/object"
	"gitlite/internal/repository"
)

var (
	// ErrRemoteNotConfigured indicates no matching remote repository is found.
	ErrRemoteNotConfigured = errors.New("remote repository not configured")
	// ErrAuthRequired indicates an access token is required to authenticate.
	ErrAuthRequired = errors.New("github personal access token required for push")
	// DefaultBaseURL is the default API host for GitHub.
	DefaultBaseURL = "https://api.github.com"
)

// GitHubClient handles communication with GitHub's Git Data API.
type GitHubClient struct {
	HTTPClient *http.Client
	BaseURL    string // e.g. "https://api.github.com"
	Owner      string
	Repo       string
	Token      string
}

// ParseGitHubURL extracts the owner and repository name from a GitHub HTTPS or SSH URL.
func ParseGitHubURL(rawURL string) (owner, repo string, err error) {
	rawURL = strings.TrimSpace(rawURL)
	if strings.HasPrefix(rawURL, "git@github.com:") {
		path := strings.TrimPrefix(rawURL, "git@github.com:")
		path = strings.TrimSuffix(path, ".git")
		parts := strings.Split(path, "/")
		if len(parts) == 2 && parts[0] != "" && parts[1] != "" {
			return parts[0], parts[1], nil
		}
		return "", "", fmt.Errorf("invalid SSH GitHub URL %q", rawURL)
	}

	u, err := url.Parse(rawURL)
	if err != nil {
		return "", "", fmt.Errorf("invalid URL %q: %w", rawURL, err)
	}

	path := strings.Trim(u.Path, "/")
	path = strings.TrimSuffix(path, ".git")
	parts := strings.Split(path, "/")
	if len(parts) >= 2 && parts[0] != "" && parts[1] != "" {
		return parts[0], parts[1], nil
	}

	return "", "", fmt.Errorf("could not extract owner and repo from %q", rawURL)
}

// NewGitHubClient creates a new client for a given owner, repo, and optional token.
func NewGitHubClient(owner, repo, token string) *GitHubClient {
	return &GitHubClient{
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
		BaseURL:    DefaultBaseURL,
		Owner:      owner,
		Repo:       repo,
		Token:      token,
	}
}

// doRequest performs an HTTP request with GitHub API headers and authentication.
func (c *GitHubClient) doRequest(method, endpoint string, body interface{}) (*http.Response, error) {
	apiURL := fmt.Sprintf("%s/repos/%s/%s%s", c.BaseURL, c.Owner, c.Repo, endpoint)

	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshaling request payload: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, apiURL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("creating http request: %w", err)
	}

	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "GitLite-Client")
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing github api request: %w", err)
	}
	return resp, nil
}

// PushResult summarizes what was sent to GitHub.
type PushResult struct {
	Branch        string `json:"branch"`
	HeadCommit    string `json:"head_commit"`
	PushedBlobs   int    `json:"pushed_blobs"`
	PushedTrees   int    `json:"pushed_trees"`
	PushedCommits int    `json:"pushed_commits"`
	RemoteRef     string `json:"remote_ref"`
	RepoURL       string `json:"repo_url"`
}

// PushOptions specifies parameters for a push operation.
type PushOptions struct {
	RemoteName  string
	BranchName  string
	Token       string
	Force       bool
	SetUpstream bool
}

// Push sends local commits, trees, and blobs to the remote GitHub repository.
func Push(repo *repository.Repository, remoteName, branchName, tokenOverride string) (*PushResult, error) {
	return PushWithOptions(repo, PushOptions{
		RemoteName: remoteName,
		BranchName: branchName,
		Token:      tokenOverride,
	})
}

// PushWithOptions sends local commits, trees, and blobs with full option support (--force, --set-upstream).
func PushWithOptions(repo *repository.Repository, opts PushOptions) (*PushResult, error) {
	rem, err := repo.GetRemote(opts.RemoteName)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrRemoteNotConfigured, opts.RemoteName)
	}

	owner, repoName, err := ParseGitHubURL(rem.URL)
	if err != nil {
		return nil, err
	}

	token := opts.Token
	if token == "" {
		token = rem.Token
	}
	if token == "" {
		token = os.Getenv("GITHUB_TOKEN")
	}
	if token == "" {
		token = os.Getenv("GH_TOKEN")
	}
	if token == "" {
		return nil, ErrAuthRequired
	}

	client := NewGitHubClient(owner, repoName, token)
	client.BaseURL = DefaultBaseURL
	res, err := PushWithClientOptions(repo, client, opts.BranchName, opts.Force)
	if err != nil {
		return nil, err
	}

	// Handle --set-upstream: store branch tracking config in .gitlite/config
	if opts.SetUpstream {
		_ = repo.SetConfigValue(fmt.Sprintf("branch.%s.remote", res.Branch), opts.RemoteName)
		_ = repo.SetConfigValue(fmt.Sprintf("branch.%s.merge", res.Branch), fmt.Sprintf("refs/heads/%s", res.Branch))
	}

	return res, nil
}

// PushWithClient pushes commits using the provided GitHub client.
func PushWithClient(repo *repository.Repository, client *GitHubClient, branchName string) (*PushResult, error) {
	return PushWithClientOptions(repo, client, branchName, false)
}

// PushWithClientOptions pushes commits using the provided GitHub client with optional force.
func PushWithClientOptions(repo *repository.Repository, client *GitHubClient, branchName string, force bool) (*PushResult, error) {
	refMgr := repo.Refs()
	if branchName == "" {
		_, curr, err := refMgr.ResolveHead()
		if err != nil {
			return nil, fmt.Errorf("determining current branch: %w", err)
		}
		if curr == "" {
			return nil, fmt.Errorf("cannot push detached HEAD; checkout a branch first")
		}
		branchName = curr
	}

	headCommitHash, err := refMgr.ResolveBranch(branchName)
	if err != nil {
		return nil, fmt.Errorf("branch %q has no commits to push: %w", branchName, err)
	}

	// 1. Gather all commits reachable from branch
	var commitsToPush []object.Hash
	visitedCommits := make(map[object.Hash]bool)
	queue := []object.Hash{headCommitHash}
	visitedCommits[headCommitHash] = true

	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]
		commitsToPush = append(commitsToPush, curr)

		cObj, err := repo.Store.ReadCommit(curr)
		if err != nil {
			return nil, fmt.Errorf("reading commit %s: %w", curr, err)
		}

		for _, p := range cObj.Parents {
			if !visitedCommits[p] {
				visitedCommits[p] = true
				queue = append(queue, p)
			}
		}
	}

	// Reverse to push oldest to newest
	for i, j := 0, len(commitsToPush)-1; i < j; i, j = i+1, j-1 {
		commitsToPush[i], commitsToPush[j] = commitsToPush[j], commitsToPush[i]
	}

	// 2. Track pushed objects
	pushedBlobs := 0
	pushedTrees := 0
	pushedCommits := 0

	syncedBlobs := make(map[object.Hash]bool)
	syncedTrees := make(map[object.Hash]bool)

	// Helper to push blob
	pushBlob := func(blobHash object.Hash) error {
		if syncedBlobs[blobHash] {
			return nil
		}
		blobObj, err := repo.Store.ReadBlob(blobHash)
		if err != nil {
			return fmt.Errorf("reading blob %s: %w", blobHash, err)
		}

		encoded := base64.StdEncoding.EncodeToString(blobObj.Data)
		payload := map[string]string{
			"content":  encoded,
			"encoding": "base64",
		}

		resp, err := client.doRequest(http.MethodPost, "/git/blobs", payload)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
			bodyBytes, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("failed pushing blob %s (status %d): %s", blobHash, resp.StatusCode, string(bodyBytes))
		}

		syncedBlobs[blobHash] = true
		pushedBlobs++
		return nil
	}

	// Helper to push tree recursively
	var pushTree func(treeHash object.Hash) error
	pushTree = func(treeHash object.Hash) error {
		if syncedTrees[treeHash] {
			return nil
		}

		tObj, err := repo.Store.ReadTree(treeHash)
		if err != nil {
			return fmt.Errorf("reading tree %s: %w", treeHash, err)
		}

		type gitTreeEntry struct {
			Path string `json:"path"`
			Mode string `json:"mode"`
			Type string `json:"type"`
			Sha  string `json:"sha"`
		}

		var treeEntries []gitTreeEntry
		for _, entry := range tObj.Entries {
			entryType := "blob"
			if entry.Mode == object.ModeDirectory {
				entryType = "tree"
				if err := pushTree(entry.Hash); err != nil {
					return err
				}
			} else {
				if err := pushBlob(entry.Hash); err != nil {
					return err
				}
			}

			treeEntries = append(treeEntries, gitTreeEntry{
				Path: entry.Name,
				Mode: string(entry.Mode),
				Type: entryType,
				Sha:  string(entry.Hash),
			})
		}

		payload := map[string]interface{}{
			"tree": treeEntries,
		}

		resp, err := client.doRequest(http.MethodPost, "/git/trees", payload)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
			bodyBytes, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("failed pushing tree %s (status %d): %s", treeHash, resp.StatusCode, string(bodyBytes))
		}

		syncedTrees[treeHash] = true
		pushedTrees++
		return nil
	}

	// 3. For each commit: ensure tree is pushed, then push commit
	for _, cHash := range commitsToPush {
		cObj, err := repo.Store.ReadCommit(cHash)
		if err != nil {
			return nil, err
		}

		if err := pushTree(cObj.TreeHash); err != nil {
			return nil, err
		}

		var parents []string
		for _, p := range cObj.Parents {
			parents = append(parents, string(p))
		}

		type gitCommitAuthor struct {
			Name  string `json:"name"`
			Email string `json:"email"`
			Date  string `json:"date"`
		}

		payload := map[string]interface{}{
			"message": cObj.Message,
			"tree":    string(cObj.TreeHash),
			"parents": parents,
			"author": gitCommitAuthor{
				Name:  cObj.Author.Name,
				Email: cObj.Author.Email,
				Date:  cObj.Author.When.Format(time.RFC3339),
			},
			"committer": gitCommitAuthor{
				Name:  cObj.Committer.Name,
				Email: cObj.Committer.Email,
				Date:  cObj.Committer.When.Format(time.RFC3339),
			},
		}

		resp, err := client.doRequest(http.MethodPost, "/git/commits", payload)
		if err != nil {
			return nil, fmt.Errorf("pushing commit %s: %w", cHash, err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
			bodyBytes, _ := io.ReadAll(resp.Body)
			return nil, fmt.Errorf("failed pushing commit %s (status %d): %s", cHash, resp.StatusCode, string(bodyBytes))
		}
		pushedCommits++
	}

	// 4. Update the branch ref on GitHub (refs/heads/<branch>)
	refEndpoint := fmt.Sprintf("/git/refs/heads/%s", branchName)
	updatePayload := map[string]interface{}{
		"sha":   string(headCommitHash),
		"force": force,
	}

	resp, err := client.doRequest(http.MethodPatch, refEndpoint, updatePayload)
	if err != nil {
		return nil, fmt.Errorf("updating ref on github: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		// Branch doesn't exist yet, create it with POST /git/refs
		createPayload := map[string]interface{}{
			"ref": fmt.Sprintf("refs/heads/%s", branchName),
			"sha": string(headCommitHash),
		}
		createResp, err := client.doRequest(http.MethodPost, "/git/refs", createPayload)
		if err != nil {
			return nil, fmt.Errorf("creating remote ref: %w", err)
		}
		defer createResp.Body.Close()
		if createResp.StatusCode != http.StatusCreated && createResp.StatusCode != http.StatusOK {
			bBytes, _ := io.ReadAll(createResp.Body)
			return nil, fmt.Errorf("failed creating remote branch %s: %s", branchName, string(bBytes))
		}
	} else if resp.StatusCode != http.StatusOK {
		bBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed updating remote branch %s: %s", branchName, string(bBytes))
	}

	return &PushResult{
		Branch:        branchName,
		HeadCommit:    string(headCommitHash),
		PushedBlobs:   pushedBlobs,
		PushedTrees:   pushedTrees,
		PushedCommits: pushedCommits,
		RemoteRef:     fmt.Sprintf("refs/heads/%s", branchName),
		RepoURL:       fmt.Sprintf("https://github.com/%s/%s", client.Owner, client.Repo),
	}, nil
}
