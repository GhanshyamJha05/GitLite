package lfs

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

const (
	// SpecVersion is the official Git LFS specification header line.
	SpecVersion = "version https://git-lfs.github.com/spec/v1"
)

var (
	// ErrInvalidPointer indicates the content is not a valid Git LFS pointer file.
	ErrInvalidPointer = errors.New("invalid Git LFS pointer format")
)

// Pointer represents a parsed Git LFS pointer specification.
type Pointer struct {
	OID  string `json:"oid"`  // SHA-256 hash
	Size int64  `json:"size"` // File size in bytes
}

// IsPointer determines if the given byte slice begins with the Git LFS spec version header.
func IsPointer(data []byte) bool {
	return bytes.HasPrefix(bytes.TrimSpace(data), []byte(SpecVersion))
}

// FormatPointer returns standard 3-line Git LFS pointer file bytes.
func FormatPointer(oid string, size int64) []byte {
	return []byte(fmt.Sprintf("%s\noid sha256:%s\nsize %d\n", SpecVersion, oid, size))
}

// ParsePointer parses a Git LFS pointer file and extracts its OID and size.
func ParsePointer(data []byte) (*Pointer, error) {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	p := &Pointer{}

	foundVersion := false
	foundOID := false
	foundSize := false

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "version ") {
			if line != SpecVersion {
				return nil, fmt.Errorf("%w: unsupported version %q", ErrInvalidPointer, line)
			}
			foundVersion = true
		} else if strings.HasPrefix(line, "oid sha256:") {
			p.OID = strings.TrimPrefix(line, "oid sha256:")
			foundOID = true
		} else if strings.HasPrefix(line, "size ") {
			sStr := strings.TrimPrefix(line, "size ")
			sz, err := strconv.ParseInt(sStr, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("%w: invalid size %q: %v", ErrInvalidPointer, sStr, err)
			}
			p.Size = sz
			foundSize = true
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	if !foundVersion || !foundOID || !foundSize {
		return nil, ErrInvalidPointer
	}

	return p, nil
}
