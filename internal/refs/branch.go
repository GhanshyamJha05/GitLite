package refs

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gitlite/internal/object"
)

var (
	// ErrBranchAlreadyExists is returned when trying to create a branch that already exists.
	ErrBranchAlreadyExists = fmt.Errorf("a branch with that name already exists")

	// ErrCannotDeleteCurrentBranch is returned when trying to delete the currently checked-out branch.
	ErrCannotDeleteCurrentBranch = fmt.Errorf("cannot delete the currently checked-out branch")
)

// BranchInfo contains metadata about a branch reference.
type BranchInfo struct {
	Name      string
	Commit    object.Hash
	IsCurrent bool
}

// ListBranches returns all branches in refs/heads sorted alphabetically.
func (m *Manager) ListBranches() ([]BranchInfo, error) {
	headsDir := filepath.Join(m.gitliteDir, "refs", "heads")
	if _, err := os.Stat(headsDir); os.IsNotExist(err) {
		return nil, nil
	}

	_, currentBranch, _ := m.ResolveHead()

	var branches []BranchInfo

	err := filepath.WalkDir(headsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(headsDir, path)
		if err != nil {
			return err
		}
		name := filepath.ToSlash(relPath)

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		h := object.Hash(strings.TrimSpace(string(data)))
		branches = append(branches, BranchInfo{
			Name:      name,
			Commit:    h,
			IsCurrent: name == currentBranch,
		})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("listing branches: %w", err)
	}

	sort.Slice(branches, func(i, j int) bool {
		return branches[i].Name < branches[j].Name
	})

	return branches, nil
}

// CreateBranch creates a new branch pointing to the specified commit hash.
func (m *Manager) CreateBranch(name string, targetCommit object.Hash) error {
	if err := targetCommit.Validate(); err != nil {
		return fmt.Errorf("validating target commit: %w", err)
	}

	branchPath := filepath.Join(m.gitliteDir, "refs", "heads", filepath.FromSlash(name))
	if _, err := os.Stat(branchPath); err == nil {
		return fmt.Errorf("%w: %q", ErrBranchAlreadyExists, name)
	}

	return m.atomicWriteRef(branchPath, targetCommit)
}

// DeleteBranch deletes a branch reference file.
func (m *Manager) DeleteBranch(name string) error {
	_, currentBranch, _ := m.ResolveHead()
	if name == currentBranch {
		return ErrCannotDeleteCurrentBranch
	}

	branchPath := filepath.Join(m.gitliteDir, "refs", "heads", filepath.FromSlash(name))
	if _, err := os.Stat(branchPath); os.IsNotExist(err) {
		return fmt.Errorf("%w: %q", ErrRefNotFound, name)
	}

	return os.Remove(branchPath)
}

// SetHeadBranch updates HEAD to point symbolically to a branch (ref: refs/heads/<branch>).
func (m *Manager) SetHeadBranch(name string) error {
	headPath := filepath.Join(m.gitliteDir, "HEAD")
	content := fmt.Sprintf("ref: refs/heads/%s\n", name)
	return os.WriteFile(headPath, []byte(content), 0644)
}

// SetHeadDetached updates HEAD to point directly to a commit hash (detached HEAD state).
func (m *Manager) SetHeadDetached(h object.Hash) error {
	if err := h.Validate(); err != nil {
		return fmt.Errorf("validating commit hash: %w", err)
	}
	headPath := filepath.Join(m.gitliteDir, "HEAD")
	content := fmt.Sprintf("%s\n", h)
	return os.WriteFile(headPath, []byte(content), 0644)
}

// ResolveBranch resolves a branch name to its commit hash.
func (m *Manager) ResolveBranch(name string) (object.Hash, error) {
	branchPath := filepath.Join(m.gitliteDir, "refs", "heads", filepath.FromSlash(name))
	data, err := os.ReadFile(branchPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("%w: %q", ErrRefNotFound, name)
		}
		return "", err
	}

	h := object.Hash(strings.TrimSpace(string(data)))
	if err := h.Validate(); err != nil {
		return "", fmt.Errorf("invalid hash in branch %s: %w", name, err)
	}
	return h, nil
}
