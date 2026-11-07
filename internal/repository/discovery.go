package repository

import (
	"fmt"
	"os"
	"path/filepath"

	"gitlite/internal/object"
)

// Discover searches upwards from startDir to locate the enclosing .gitlite repository.
// If startDir is empty, the current working directory is used.
func Discover(startDir string) (*Repository, error) {
	if startDir == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("getting current working directory: %w", err)
		}
		startDir = cwd
	}

	curr, err := filepath.Abs(startDir)
	if err != nil {
		return nil, fmt.Errorf("resolving absolute path: %w", err)
	}

	for {
		gitlitePath := filepath.Join(curr, DotGitLite)
		info, err := os.Stat(gitlitePath)
		if err == nil && info.IsDir() {
			return &Repository{
				WorkTree:   curr,
				GitLiteDir: gitlitePath,
				Store:      object.NewStore(filepath.Join(gitlitePath, ObjectsDir)),
			}, nil
		}

		parent := filepath.Dir(curr)
		if parent == curr {
			// Reached filesystem root
			break
		}
		curr = parent
	}

	return nil, ErrRepositoryNotFound
}
