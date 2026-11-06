package repository

import (
	"errors"
	"fmt"
	"os"
	"os/user"
	"time"

	"gitlite/internal/object"
	"gitlite/internal/refs"
)

var (
	// ErrNothingToCommit is returned when the staging index is empty.
	ErrNothingToCommit = errors.New("nothing to commit (create/copy files and use 'gitlite add' to track)")
)

// Refs returns the reference manager bound to this repository.
func (r *Repository) Refs() *refs.Manager {
	return refs.NewManager(r.GitLiteDir)
}

// DefaultSignature returns an author/committer signature based on system or environment configuration.
func DefaultSignature() object.Signature {
	name := os.Getenv("GITLITE_AUTHOR_NAME")
	email := os.Getenv("GITLITE_AUTHOR_EMAIL")

	if name == "" {
		if u, err := user.Current(); err == nil && u.Username != "" {
			name = u.Username
		} else {
			name = "GitLite User"
		}
	}

	if email == "" {
		email = "user@gitlite.local"
	}

	return object.Signature{
		Name:  name,
		Email: email,
		When:  time.Now(),
	}
}

// Commit creates an immutable snapshot of the current staged index, links parent commit(s),
// writes the Commit object to CAS, and advances HEAD/branch atomically.
func (r *Repository) Commit(message string, author, committer object.Signature) (object.Hash, error) {
	if message == "" {
		return "", errors.New("commit message cannot be empty")
	}

	idx, err := r.LoadIndex()
	if err != nil {
		return "", fmt.Errorf("loading index: %w", err)
	}

	if len(idx.Entries) == 0 {
		return "", ErrNothingToCommit
	}

	// 1. Build and write root tree object
	rootTreeHash, err := idx.WriteTree(r.Store)
	if err != nil {
		return "", fmt.Errorf("writing tree from index: %w", err)
	}

	// 2. Discover parent commit from HEAD
	refMgr := r.Refs()
	headCommit, _, err := refMgr.ResolveHead()
	var parents []object.Hash
	if err == nil && headCommit != "" {
		// Existing history
		parents = append(parents, headCommit)
	}

	// 3. Construct and write Commit object
	commitObj := object.NewCommit(rootTreeHash, parents, author, committer, message)
	commitHash, err := r.Store.Write(commitObj)
	if err != nil {
		return "", fmt.Errorf("writing commit object: %w", err)
	}

	// 4. Atomically update HEAD (or active branch)
	if err := refMgr.UpdateHead(commitHash); err != nil {
		return "", fmt.Errorf("updating HEAD reference: %w", err)
	}

	return commitHash, nil
}

// Log traverses the commit graph starting from the commit at HEAD, returning history in reverse chronological order.
func (r *Repository) Log(startCommit object.Hash) ([]*object.CommitWithHash, error) {
	var result []*object.CommitWithHash
	currHash := startCommit

	visited := make(map[object.Hash]bool)

	for currHash != "" && !visited[currHash] {
		visited[currHash] = true

		commitObj, err := r.Store.ReadCommit(currHash)
		if err != nil {
			return nil, fmt.Errorf("reading commit %s: %w", currHash, err)
		}

		result = append(result, &object.CommitWithHash{
			Hash:   currHash,
			Commit: commitObj,
		})

		if len(commitObj.Parents) == 0 {
			break
		}
		// Follow first parent for linear log history
		currHash = commitObj.Parents[0]
	}

	return result, nil
}
