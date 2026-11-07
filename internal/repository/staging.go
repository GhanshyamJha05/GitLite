package repository

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"gitlite/internal/index"
	"gitlite/internal/object"
)

// IndexPath returns the absolute path to .gitlite/index.
func (r *Repository) IndexPath() string {
	return filepath.Join(r.GitLiteDir, index.IndexFile)
}

// LoadIndex opens and returns the repository's staging index.
func (r *Repository) LoadIndex() (*index.Index, error) {
	return index.Load(r.IndexPath())
}

// SaveIndex persists the provided index atomically to disk.
func (r *Repository) SaveIndex(idx *index.Index) error {
	return idx.Save(r.IndexPath())
}

// Add stages the file or directory at relOrAbsPath.
// If the path is a directory, it recursively traverses and stages all non-ignored files.
func (r *Repository) Add(relOrAbsPath string) error {
	targetAbs, err := filepath.Abs(relOrAbsPath)
	if err != nil {
		return fmt.Errorf("resolving target path: %w", err)
	}

	// Security: Prevent path traversal outside the repository work tree
	relToWorkTree, err := filepath.Rel(r.WorkTree, targetAbs)
	if err != nil || strings.HasPrefix(relToWorkTree, "..") {
		return fmt.Errorf("path %s is outside repository worktree %s", relOrAbsPath, r.WorkTree)
	}

	// Prevent staging files inside .gitlite itself
	if relToWorkTree == DotGitLite || strings.HasPrefix(relToWorkTree, DotGitLite+string(filepath.Separator)) {
		return nil
	}

	info, err := os.Stat(targetAbs)
	if err != nil {
		return fmt.Errorf("stat target: %w", err)
	}

	idx, err := r.LoadIndex()
	if err != nil {
		return fmt.Errorf("loading index: %w", err)
	}

	if info.IsDir() {
		// Walk directory recursively
		err = filepath.WalkDir(targetAbs, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}

			// Skip .gitlite directory
			if d.IsDir() && d.Name() == DotGitLite {
				return filepath.SkipDir
			}

			if d.IsDir() {
				return nil
			}

			return r.stageFile(idx, path)
		})
		if err != nil {
			return fmt.Errorf("walking directory %s: %w", targetAbs, err)
		}
	} else {
		if err := r.stageFile(idx, targetAbs); err != nil {
			return err
		}
	}

	return r.SaveIndex(idx)
}

// stageFile reads a single file from disk, creates a blob in the object store, and adds an Entry to idx.
func (r *Repository) stageFile(idx *index.Index, absFilePath string) error {
	relPath, err := filepath.Rel(r.WorkTree, absFilePath)
	if err != nil {
		return fmt.Errorf("computing relative path: %w", err)
	}

	normRelPath := filepath.ToSlash(relPath)
	if strings.HasPrefix(normRelPath, DotGitLite+"/") || normRelPath == DotGitLite {
		return nil
	}

	info, err := os.Stat(absFilePath)
	if err != nil {
		return fmt.Errorf("stating file %s: %w", absFilePath, err)
	}

	data, err := os.ReadFile(absFilePath)
	if err != nil {
		return fmt.Errorf("reading file %s: %w", absFilePath, err)
	}

	blob := object.NewBlob(data)
	h, err := r.Store.Write(blob)
	if err != nil {
		return fmt.Errorf("writing blob for %s: %w", absFilePath, err)
	}

	mode := object.ModeRegular
	// Check if file is executable (Unix permission 0111)
	if info.Mode()&0111 != 0 {
		mode = object.ModeExecutable
	}

	idx.Add(index.Entry{
		Path:        normRelPath,
		Hash:        h,
		Mode:        mode,
		Size:        info.Size(),
		ModTimeNano: info.ModTime().UnixNano(),
	})

	return nil
}
