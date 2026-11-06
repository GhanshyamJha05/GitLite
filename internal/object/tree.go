package object

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
)

// FileMode represents POSIX file permissions for tree entries.
type FileMode string

const (
	// ModeRegular represents a standard regular file (0644).
	ModeRegular FileMode = "100644"

	// ModeExecutable represents an executable regular file (0755).
	ModeExecutable FileMode = "100755"

	// ModeDirectory represents a subdirectory tree (040000).
	ModeDirectory FileMode = "040000"
)

// TreeEntry represents an item inside a Tree (either a blob or another subtree).
type TreeEntry struct {
	Mode FileMode
	Name string
	Hash Hash
}

// Tree represents a directory snapshot containing entries with deterministic sorting.
type Tree struct {
	Entries []TreeEntry
}

// NewTree creates a new empty Tree.
func NewTree(entries []TreeEntry) *Tree {
	t := &Tree{
		Entries: entries,
	}
	t.Sort()
	return t
}

// Type returns TypeTree.
func (t *Tree) Type() Type {
	return TypeTree
}

// Sort orders tree entries deterministically by Name (POSIX byte-order sorting).
func (t *Tree) Sort() {
	sort.Slice(t.Entries, func(i, j int) bool {
		return t.Entries[i].Name < t.Entries[j].Name
	})
}

// SerializeContent formats tree entries canonically.
// Format per entry:
// <mode> <name>\0<32-byte-binary-hash>
func (t *Tree) SerializeContent() []byte {
	t.Sort()
	var buf bytes.Buffer
	for _, entry := range t.Entries {
		hashBytes, err := hex.DecodeString(string(entry.Hash))
		if err != nil || len(hashBytes) != 32 {
			continue
		}
		// Write: "<mode> <name>\0<32-byte-hash>"
		buf.WriteString(fmt.Sprintf("%s %s\x00", entry.Mode, entry.Name))
		buf.Write(hashBytes)
	}
	return buf.Bytes()
}

// ParseTree parses the raw uncompressed content payload of a Tree object into a *Tree.
func ParseTree(data []byte) (*Tree, error) {
	var entries []TreeEntry
	i := 0
	n := len(data)

	for i < n {
		// Find space separating mode and name
		spaceIdx := bytes.IndexByte(data[i:], ' ')
		if spaceIdx == -1 {
			return nil, fmt.Errorf("%w: tree entry missing space delimiter", ErrCorruptObject)
		}
		mode := FileMode(data[i : i+spaceIdx])
		i += spaceIdx + 1

		// Find null byte separating name and 32-byte hash
		nullIdx := bytes.IndexByte(data[i:], 0)
		if nullIdx == -1 {
			return nil, fmt.Errorf("%w: tree entry missing null terminator", ErrCorruptObject)
		}
		name := string(data[i : i+nullIdx])
		i += nullIdx + 1

		// Extract 32-byte binary SHA-256 hash
		if i+32 > n {
			return nil, fmt.Errorf("%w: truncated hash in tree entry %q", ErrCorruptObject, name)
		}
		rawHash := data[i : i+32]
		i += 32

		h := Hash(hex.EncodeToString(rawHash))
		if err := h.Validate(); err != nil {
			return nil, fmt.Errorf("%w: invalid hash in tree entry %q: %v", ErrCorruptObject, name, err)
		}

		entries = append(entries, TreeEntry{
			Mode: mode,
			Name: name,
			Hash: h,
		})
	}

	return NewTree(entries), nil
}

// ReadTree retrieves, verifies, and parses a Tree object from the store.
func (s *Store) ReadTree(h Hash) (*Tree, error) {
	objType, content, err := s.Read(h)
	if err != nil {
		return nil, err
	}
	if objType != TypeTree {
		return nil, fmt.Errorf("expected object %s to be tree, got %s", h, objType)
	}
	return ParseTree(content)
}

// String formats the tree for human-readable debugging.
func (t *Tree) String() string {
	var sb strings.Builder
	for _, entry := range t.Entries {
		sb.WriteString(fmt.Sprintf("%s %s %s\n", entry.Mode, entry.Hash, entry.Name))
	}
	return sb.String()
}
