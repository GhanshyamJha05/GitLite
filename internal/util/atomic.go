package util

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// AtomicWriteFile safely writes content to targetPath by writing to a temporary file
// in the same directory, closing it, and renaming it to targetPath with a retry mechanism
// for Windows file-system locking.
func AtomicWriteFile(targetPath string, content []byte, perm os.FileMode) error {
	parent := filepath.Dir(targetPath)
	if err := os.MkdirAll(parent, 0755); err != nil {
		return fmt.Errorf("creating parent directory %s: %w", parent, err)
	}

	tmpFile, err := os.CreateTemp(parent, "tmp_atomic_*")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpName := tmpFile.Name()
	defer func() {
		_ = os.Remove(tmpName)
	}()

	if _, err := tmpFile.Write(content); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("writing content: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("closing temp file: %w", err)
	}

	// Retry loop for Windows transient file locks (antivirus, search indexer)
	var renameErr error
	for attempts := 0; attempts < 5; attempts++ {
		renameErr = os.Rename(tmpName, targetPath)
		if renameErr == nil {
			return nil
		}
		time.Sleep(10 * time.Millisecond)
	}

	return fmt.Errorf("committing file %s: %w", targetPath, renameErr)
}
