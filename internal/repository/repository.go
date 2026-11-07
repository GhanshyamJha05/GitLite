package repository

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gitlite/internal/object"
)

const (
	// DotGitLite is the name of the GitLite repository directory.
	DotGitLite = ".gitlite"

	// ObjectsDir is the directory storing content-addressed objects.
	ObjectsDir = "objects"

	// RefsDir is the directory storing references.
	RefsDir = "refs"

	// HeadsDir is the subdirectory of refs storing branch pointers.
	HeadsDir = "refs/heads"

	// HeadFile is the file tracking the active branch or detached commit.
	HeadFile = "HEAD"

	// ConfigFile is the repository configuration file.
	ConfigFile = "config"

	// DefaultBranch is the default branch created on repository initialization.
	DefaultBranch = "main"

	// DefaultInitialConfig is the baseline configuration JSON stored at init.
	DefaultInitialConfig = "{\n  \"version\": 1\n}\n"
)

var (
	// ErrAlreadyInitialized indicates a GitLite repository already exists at the target path.
	ErrAlreadyInitialized = errors.New("repository already initialized")

	// ErrRepositoryNotFound indicates no GitLite repository was found when discovering roots.
	ErrRepositoryNotFound = errors.New("not a gitlite repository (or any of the parent directories)")
)

// Repository represents an open GitLite repository instance.
type Repository struct {
	// WorkTree is the root of the user's working directory.
	WorkTree string
	// GitLiteDir is the absolute path to .gitlite directory.
	GitLiteDir string
	// Store provides access to the immutable content-addressable object store.
	Store *object.Store
}

// Init creates a new GitLite repository at targetPath.
// If targetPath is empty, it initializes in the current working directory.
// Returns an error if targetPath already contains a .gitlite directory.
func Init(targetPath string) (*Repository, error) {
	if targetPath == "" {
		targetPath = "."
	}

	absPath, err := filepath.Abs(targetPath)
	if err != nil {
		return nil, fmt.Errorf("resolving absolute path: %w", err)
	}

	gitliteDir := filepath.Join(absPath, DotGitLite)

	// Check idempotency - must not overwrite existing repository
	if _, err := os.Stat(gitliteDir); err == nil {
		return nil, fmt.Errorf("%w: %s", ErrAlreadyInitialized, gitliteDir)
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("checking repository directory: %w", err)
	}

	// Create directory structure safely
	dirs := []string{
		gitliteDir,
		filepath.Join(gitliteDir, ObjectsDir),
		filepath.Join(gitliteDir, filepath.FromSlash(HeadsDir)),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("creating directory %s: %w", dir, err)
		}
	}

	// Initialize HEAD pointing to default branch ref
	// e.g. ref: refs/heads/main
	headPath := filepath.Join(gitliteDir, HeadFile)
	initialHeadContent := fmt.Sprintf("ref: refs/heads/%s\n", DefaultBranch)
	if err := os.WriteFile(headPath, []byte(initialHeadContent), 0644); err != nil {
		return nil, fmt.Errorf("writing HEAD file: %w", err)
	}

	// Initialize config file
	configPath := filepath.Join(gitliteDir, ConfigFile)
	if err := os.WriteFile(configPath, []byte(DefaultInitialConfig), 0644); err != nil {
		return nil, fmt.Errorf("writing config file: %w", err)
	}

	return &Repository{
		WorkTree:   absPath,
		GitLiteDir: gitliteDir,
		Store:      object.NewStore(filepath.Join(gitliteDir, ObjectsDir)),
	}, nil
}
