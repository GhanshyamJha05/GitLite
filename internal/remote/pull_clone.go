package remote

import (
	"encoding/base64"
	"errors"
	"fmt"
	"os"

	"gitlite/internal/object"
	"gitlite/internal/repository"
)

// Pull fetches changes from remote and performs a fast-forward or 3-way merge into the current branch.
func Pull(repo *repository.Repository, remoteName, branchName, tokenOverride string) (*FetchResult, error) {
	fetchRes, err := Fetch(repo, remoteName, tokenOverride)
	if err != nil {
		return nil, fmt.Errorf("fetch failed: %w", err)
	}

	refMgr := repo.Refs()
	if branchName == "" {
		_, curr, err := refMgr.ResolveHead()
		if err != nil {
			return nil, fmt.Errorf("resolving current branch: %w", err)
		}
		if curr == "" {
			return nil, fmt.Errorf("cannot pull into detached HEAD; checkout a branch first")
		}
		branchName = curr
	}

	remoteCommitSha, exists := fetchRes.UpdatedRefs[branchName]
	if !exists {
		return nil, fmt.Errorf("remote branch %q not found in fetched refs", branchName)
	}

	// Update local branch pointer or merge
	localCommit, err := refMgr.ResolveBranch(branchName)
	if err != nil || localCommit == "" {
		// Local branch does not exist or has no commits: fast-forward directly
		if err := refMgr.CreateBranch(branchName, object.Hash(remoteCommitSha)); err != nil {
			// If already exists, update ref directly
			_ = refMgr.SetHeadBranch(branchName)
			_ = refMgr.UpdateHead(object.Hash(remoteCommitSha))
		}
		_ = repo.Reset(object.Hash(remoteCommitSha), repository.ResetHard)
		return fetchRes, nil
	}

	if string(localCommit) == remoteCommitSha {
		// Already up to date
		return fetchRes, nil
	}

	// Fast-forward or 3-way merge using merge engine
	mergeRes, err := repo.Merge(remoteCommitSha)
	if err != nil {
		if errors.Is(err, repository.ErrAlreadyUpToDate) {
			return fetchRes, nil
		}
		return nil, fmt.Errorf("merge failed during pull: %w", err)
	}

	if len(mergeRes.Conflicts) > 0 {
		return fetchRes, fmt.Errorf("automatic pull merge failed with conflicts in %d files", len(mergeRes.Conflicts))
	}

	return fetchRes, nil
}

// Clone clones a remote repository into targetDir, configures 'origin', and checks out the default branch.
func Clone(remoteURL, targetDir, token string) (*repository.Repository, error) {
	if targetDir == "" {
		_, repoName, err := ParseGitHubURL(remoteURL)
		if err != nil {
			return nil, err
		}
		targetDir = repoName
	}

	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return nil, fmt.Errorf("creating clone target directory: %w", err)
	}

	repo, err := repository.Init(targetDir)
	if err != nil {
		return nil, fmt.Errorf("initializing repository for clone: %w", err)
	}

	// Add remote origin
	if err := repo.AddRemote("origin", remoteURL, token); err != nil {
		return nil, fmt.Errorf("adding remote origin: %w", err)
	}

	// Fetch all branches and objects
	fetchRes, err := Fetch(repo, "origin", token)
	if err != nil {
		return nil, fmt.Errorf("cloning objects from remote: %w", err)
	}

	// Find default branch (main or master or first ref)
	defaultBranch := "main"
	targetCommit := ""
	if sha, ok := fetchRes.UpdatedRefs["main"]; ok {
		defaultBranch = "main"
		targetCommit = sha
	} else if sha, ok := fetchRes.UpdatedRefs["master"]; ok {
		defaultBranch = "master"
		targetCommit = sha
	} else {
		for b, sha := range fetchRes.UpdatedRefs {
			defaultBranch = b
			targetCommit = sha
			break
		}
	}

	if targetCommit != "" {
		_ = repo.Refs().SetHeadBranch(defaultBranch)
		_ = repo.Refs().UpdateHead(object.Hash(targetCommit))
		if err := repo.Reset(object.Hash(targetCommit), repository.ResetHard); err != nil {
			return nil, fmt.Errorf("checking out %s: %w", defaultBranch, err)
		}
		// Set upstream configuration
		_ = repo.SetConfigValue(fmt.Sprintf("branch.%s.remote", defaultBranch), "origin")
		_ = repo.SetConfigValue(fmt.Sprintf("branch.%s.merge", defaultBranch), fmt.Sprintf("refs/heads/%s", defaultBranch))
	}

	return repo, nil
}

// DecodeBase64Content safely decodes base64-encoded strings with newlines.
func DecodeBase64Content(s string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(s)
}
