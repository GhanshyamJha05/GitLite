package object

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
)

// Hash is a 32-byte SHA-256 cryptographic checksum represented as a 64-character lowercase hexadecimal string.
type Hash string

// ZeroHash represents an empty or uninitialized hash.
const ZeroHash Hash = "0000000000000000000000000000000000000000000000000000000000000000"

var hashRegex = regexp.MustCompile(`^[0-9a-f]{64}$`)

// ErrInvalidHash indicates that the hash string does not match the 64-character hexadecimal SHA-256 format.
var ErrInvalidHash = errors.New("invalid SHA-256 object hash")

// Validate checks whether the Hash is a well-formed 64-character lowercase hex string.
func (h Hash) Validate() error {
	if !hashRegex.MatchString(string(h)) {
		return fmt.Errorf("%w: %q", ErrInvalidHash, string(h))
	}
	return nil
}

// ComputeHash calculates the SHA-256 hash of the given raw bytes and returns it as a Hash.
func ComputeHash(data []byte) Hash {
	sum := sha256.Sum256(data)
	return Hash(hex.EncodeToString(sum[:]))
}

// String returns the string representation of the hash.
func (h Hash) String() string {
	return string(h)
}

