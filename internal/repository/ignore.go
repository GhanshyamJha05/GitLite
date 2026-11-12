package repository

import (
	"path/filepath"

	"gitlite/internal/ignore"
)

// IgnoreMatcher returns a Matcher loaded with root and local .gitliteignore / .gitignore rules.
func (r *Repository) IgnoreMatcher() (*ignore.Matcher, error) {
	matcher := ignore.NewMatcher()

	// Load .gitliteignore from repository root
	gitliteIgnorePath := filepath.Join(r.WorkTree, GitLiteIgnoreFile)
	if err := matcher.LoadFile(gitliteIgnorePath, ""); err != nil {
		return nil, err
	}

	// Also load .gitignore as fallback/supplement
	gitIgnorePath := filepath.Join(r.WorkTree, GitIgnoreFile)
	if err := matcher.LoadFile(gitIgnorePath, ""); err != nil {
		return nil, err
	}

	return matcher, nil
}
