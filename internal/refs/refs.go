package refs

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gitlite/internal/object"
)

var (
	// ErrRefNotFound is returned when a requested reference cannot be found.
	ErrRefNotFound = errors.New("reference not found")
)

// Manager manages reading and updating references in a repository.
type Manager struct {
	gitliteDir string
}

// NewManager creates a new reference Manager.
func NewManager(gitliteDir string) *Manager {
	return &Manager{
		gitliteDir: gitliteDir,
	}
}

// ResolveHead reads HEAD and resolves it to a commit Hash (if branch has commits)
// or returns ErrRefNotFound if on an unborn branch.
// Also returns the current branch name (or empty string if HEAD is detached).
func (m *Manager) ResolveHead() (object.Hash, string, error) {
	headPath := filepath.Join(m.gitliteDir, "HEAD")
	data, err := os.ReadFile(headPath)
	if err != nil {
		return "", "", fmt.Errorf("reading HEAD: %w", err)
	}

	content := strings.TrimSpace(string(data))
	if strings.HasPrefix(content, "ref: ") {
		refPath := strings.TrimSpace(strings.TrimPrefix(content, "ref: "))
		branchName := strings.TrimPrefix(refPath, "refs/heads/")
		targetPath := filepath.Join(m.gitliteDir, filepath.FromSlash(refPath))

		refData, err := os.ReadFile(targetPath)
		if err != nil {
			if os.IsNotExist(err) {
				// Unborn branch (e.g. init without commits yet)
				return "", branchName, ErrRefNotFound
			}
			return "", branchName, fmt.Errorf("reading ref %s: %w", refPath, err)
		}

		h := object.Hash(strings.TrimSpace(string(refData)))
		if err := h.Validate(); err != nil {
			return "", branchName, fmt.Errorf("invalid hash in ref %s: %w", refPath, err)
		}
		return h, branchName, nil
	}

	// Detached HEAD pointing directly to a commit hash
	h := object.Hash(content)
	if err := h.Validate(); err != nil {
		return "", "", fmt.Errorf("invalid hash in detached HEAD: %w", err)
	}
	return h, "", nil
}

// UpdateHead updates HEAD: if on a branch, it atomically updates the branch reference file;
// if detached, it writes the commit hash directly to HEAD.
func (m *Manager) UpdateHead(newHash object.Hash) error {
	if err := newHash.Validate(); err != nil {
		return fmt.Errorf("validating commit hash: %w", err)
	}

	headPath := filepath.Join(m.gitliteDir, "HEAD")
	data, err := os.ReadFile(headPath)
	if err != nil {
		return fmt.Errorf("reading HEAD: %w", err)
	}

	content := strings.TrimSpace(string(data))
	if strings.HasPrefix(content, "ref: ") {
		refPath := strings.TrimSpace(strings.TrimPrefix(content, "ref: "))
		targetAbs := filepath.Join(m.gitliteDir, filepath.FromSlash(refPath))
		return m.atomicWriteRef(targetAbs, newHash)
	}

	// Detached HEAD
	return m.atomicWriteRef(headPath, newHash)
}

// atomicWriteRef safely writes a ref using a temporary file and atomic rename.
func (m *Manager) atomicWriteRef(targetPath string, h object.Hash) error {
	parent := filepath.Dir(targetPath)
	if err := os.MkdirAll(parent, 0755); err != nil {
		return fmt.Errorf("creating ref directory %s: %w", parent, err)
	}

	tmpFile, err := os.CreateTemp(parent, "tmp_ref_*")
	if err != nil {
		return fmt.Errorf("creating temp ref file: %w", err)
	}
	tmpName := tmpFile.Name()
	defer func() {
		_ = os.Remove(tmpName)
	}()

	payload := fmt.Sprintf("%s\n", h)
	if _, err := tmpFile.WriteString(payload); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("writing ref payload: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("closing temp ref file: %w", err)
	}

	if err := os.Rename(tmpName, targetPath); err != nil {
		return fmt.Errorf("committing ref file: %w", err)
	}

	return nil
}

