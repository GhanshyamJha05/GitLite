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
	// ErrTagAlreadyExists is returned when attempting to create a tag that already exists.
	ErrTagAlreadyExists = fmt.Errorf("a tag with that name already exists")
	// ErrTagNotFound is returned when attempting to reference a non-existent tag.
	ErrTagNotFound = fmt.Errorf("tag not found")
)

// TagInfo holds metadata about a tag reference.
type TagInfo struct {
	Name   string      `json:"name"`
	Commit object.Hash `json:"commit"`
}

// CreateTag creates a lightweight tag pointing to targetCommit.
func (m *Manager) CreateTag(name string, targetCommit object.Hash) error {
	name = strings.TrimSpace(name)
	if err := ValidateBranchName(name); err != nil {
		return fmt.Errorf("invalid tag name %q: %w", name, err)
	}

	if err := targetCommit.Validate(); err != nil {
		return fmt.Errorf("validating commit hash: %w", err)
	}

	tagDir := filepath.Join(m.gitliteDir, "refs", "tags")
	if err := os.MkdirAll(tagDir, 0755); err != nil {
		return fmt.Errorf("creating tags directory: %w", err)
	}

	tagPath := filepath.Join(tagDir, name)
	if _, err := os.Stat(tagPath); err == nil {
		return fmt.Errorf("%w: %q", ErrTagAlreadyExists, name)
	}

	return m.atomicWriteRef(tagPath, targetCommit)
}

// DeleteTag removes a tag reference.
func (m *Manager) DeleteTag(name string) error {
	tagPath := filepath.Join(m.gitliteDir, "refs", "tags", strings.TrimSpace(name))
	if _, err := os.Stat(tagPath); os.IsNotExist(err) {
		return fmt.Errorf("%w: %q", ErrTagNotFound, name)
	}
	return os.Remove(tagPath)
}

// ResolveTag resolves a tag name to its target commit hash.
func (m *Manager) ResolveTag(name string) (object.Hash, error) {
	tagPath := filepath.Join(m.gitliteDir, "refs", "tags", strings.TrimSpace(name))
	data, err := os.ReadFile(tagPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("%w: %q", ErrTagNotFound, name)
		}
		return "", err
	}

	h := object.Hash(strings.TrimSpace(string(data)))
	if err := h.Validate(); err != nil {
		return "", fmt.Errorf("invalid hash in tag %s: %w", name, err)
	}
	return h, nil
}

// ListTags returns all tags sorted alphabetically.
func (m *Manager) ListTags() ([]TagInfo, error) {
	tagDir := filepath.Join(m.gitliteDir, "refs", "tags")
	if _, err := os.Stat(tagDir); os.IsNotExist(err) {
		return nil, nil
	}

	var tags []TagInfo
	err := filepath.WalkDir(tagDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(tagDir, path)
		if err != nil {
			return err
		}
		name := filepath.ToSlash(relPath)

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		h := object.Hash(strings.TrimSpace(string(data)))
		tags = append(tags, TagInfo{
			Name:   name,
			Commit: h,
		})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("listing tags: %w", err)
	}

	sort.Slice(tags, func(i, j int) bool {
		return tags[i].Name < tags[j].Name
	})

	return tags, nil
}
