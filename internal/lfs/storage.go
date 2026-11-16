package lfs

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"gitlite/internal/util"
)

// ObjectPath returns the relative or absolute path to an LFS payload file on disk.
func ObjectPath(gitliteDir, oid string) string {
	if len(oid) < 4 {
		return filepath.Join(gitliteDir, "lfs", "objects", oid)
	}
	return filepath.Join(gitliteDir, "lfs", "objects", oid[:2], oid[2:4], oid)
}

// Store writes an arbitrary stream to the LFS object database, calculates its SHA-256 and byte size,
// and returns the corresponding Pointer.
func Store(gitliteDir string, r io.Reader) (*Pointer, error) {
	tempDir := filepath.Join(gitliteDir, "lfs", "tmp")
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return nil, fmt.Errorf("creating lfs tmp dir: %w", err)
	}

	tempFile, err := os.CreateTemp(tempDir, "lfs-upload-*")
	if err != nil {
		return nil, fmt.Errorf("creating lfs temp file: %w", err)
	}
	defer func() {
		_ = tempFile.Close()
		_ = os.Remove(tempFile.Name())
	}()

	hasher := sha256.New()
	writer := io.MultiWriter(tempFile, hasher)

	written, err := io.Copy(writer, r)
	if err != nil {
		return nil, fmt.Errorf("writing lfs stream: %w", err)
	}

	if err := tempFile.Close(); err != nil {
		return nil, err
	}

	oid := hex.EncodeToString(hasher.Sum(nil))
	targetPath := ObjectPath(gitliteDir, oid)

	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		return nil, fmt.Errorf("creating lfs object directory: %w", err)
	}

	// Atomic persistence
	if err := os.Rename(tempFile.Name(), targetPath); err != nil {
		// If cross-device or exists on Windows, fallback to copy
		if copyErr := util.AtomicWriteFile(targetPath, nil, 0644); copyErr != nil {
			data, readErr := os.ReadFile(tempFile.Name())
			if readErr != nil {
				return nil, readErr
			}
			if writeErr := os.WriteFile(targetPath, data, 0644); writeErr != nil {
				return nil, writeErr
			}
		}
	}

	return &Pointer{
		OID:  oid,
		Size: written,
	}, nil
}

// Open opens an existing LFS object payload by its OID for reading.
func Open(gitliteDir, oid string) (io.ReadCloser, error) {
	path := ObjectPath(gitliteDir, oid)
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening lfs object %s: %w", oid, err)
	}
	return f, nil
}

// Exists checks whether the large object file exists in the LFS store.
func Exists(gitliteDir, oid string) bool {
	path := ObjectPath(gitliteDir, oid)
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
