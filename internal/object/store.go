package object

import (
	"bytes"
	"compress/zlib"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

var (
	// ErrObjectNotFound is returned when an object hash cannot be found in the store.
	ErrObjectNotFound = errors.New("object not found")

	// ErrHashMismatch is returned when an object's contents do not hash to its expected identifier.
	ErrHashMismatch = errors.New("object hash mismatch (corrupted data)")
)

// Store handles persistence, retrieval, and verification of immutable objects.
type Store struct {
	objectsDir string
}

// NewStore creates a new Store pointing to the given objects directory (e.g. .gitlite/objects).
func NewStore(objectsDir string) *Store {
	return &Store{
		objectsDir: objectsDir,
	}
}

// ObjectPath returns the relative or absolute path to an object file using 2-level directory sharding.
// e.g. objects/4a/12b6...
func (s *Store) ObjectPath(h Hash) (string, error) {
	if err := h.Validate(); err != nil {
		return "", err
	}
	dir := string(h[:2])
	file := string(h[2:])
	return filepath.Join(s.objectsDir, dir, file), nil
}

// Has checks whether an object with the specified hash exists in the store.
func (s *Store) Has(h Hash) bool {
	path, err := s.ObjectPath(h)
	if err != nil {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// Write serializes, hashes, compresses, and atomically writes an Object to disk.
// Returns the computed SHA-256 Hash.
func (s *Store) Write(obj Object) (Hash, error) {
	canonicalBytes := EncodeCanonical(obj)
	h := ComputeHash(canonicalBytes)

	// If the object already exists, content-addressable storage guarantees it is identical.
	if s.Has(h) {
		return h, nil
	}

	targetPath, err := s.ObjectPath(h)
	if err != nil {
		return "", fmt.Errorf("resolving object path: %w", err)
	}

	parentDir := filepath.Dir(targetPath)
	if err := os.MkdirAll(parentDir, 0755); err != nil {
		return "", fmt.Errorf("creating object directory %s: %w", parentDir, err)
	}

	// Crash-safe write: write to temporary file first, then atomic rename
	tmpFile, err := os.CreateTemp(parentDir, "tmp_obj_*")
	if err != nil {
		return "", fmt.Errorf("creating temporary object file: %w", err)
	}
	tmpName := tmpFile.Name()
	defer func() {
		// Clean up temporary file if rename didn't happen
		_ = os.Remove(tmpName)
	}()

	// Compress using zlib
	zw := zlib.NewWriter(tmpFile)
	if _, err := zw.Write(canonicalBytes); err != nil {
		_ = tmpFile.Close()
		return "", fmt.Errorf("compressing object: %w", err)
	}
	if err := zw.Close(); err != nil {
		_ = tmpFile.Close()
		return "", fmt.Errorf("closing zlib compressor: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return "", fmt.Errorf("closing temporary object file: %w", err)
	}

	// Atomic rename to final path with retry for Windows file-system locking
	var renameErr error
	for attempts := 0; attempts < 5; attempts++ {
		renameErr = os.Rename(tmpName, targetPath)
		if renameErr == nil {
			return h, nil
		}
		if s.Has(h) {
			return h, nil
		}
		time.Sleep(10 * time.Millisecond)
	}

	return "", fmt.Errorf("persisting object file: %w", renameErr)

	return h, nil
}

// Read decompresses and reads the raw canonical object bytes, verifies its integrity,
// and parses it into (Type, content, error).
func (s *Store) Read(h Hash) (Type, []byte, error) {
	path, err := s.ObjectPath(h)
	if err != nil {
		return "", nil, err
	}

	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil, fmt.Errorf("%w: %s", ErrObjectNotFound, h)
		}
		return "", nil, fmt.Errorf("opening object file: %w", err)
	}
	defer f.Close()

	zr, err := zlib.NewReader(f)
	if err != nil {
		return "", nil, fmt.Errorf("%w: failed to decompress object %s: %v", ErrCorruptObject, h, err)
	}
	defer zr.Close()

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, zr); err != nil {
		return "", nil, fmt.Errorf("%w: reading decompressed object stream %s: %v", ErrCorruptObject, h, err)
	}

	rawBytes := buf.Bytes()

	// Verify cryptographic integrity
	actualHash := ComputeHash(rawBytes)
	if actualHash != h {
		return "", nil, fmt.Errorf("%w: expected %s, got %s", ErrHashMismatch, h, actualHash)
	}

	return DecodeCanonical(rawBytes)
}

// ReadBlob retrieves and decodes a Blob object.
func (s *Store) ReadBlob(h Hash) (*Blob, error) {
	objType, content, err := s.Read(h)
	if err != nil {
		return nil, err
	}
	if objType != TypeBlob {
		return nil, fmt.Errorf("expected object %s to be blob, got %s", h, objType)
	}
	return NewBlob(content), nil
}

// Verify checks decompression, hash integrity, and header parsing for an object.
func (s *Store) Verify(h Hash) error {
	_, _, err := s.Read(h)
	return err
}
