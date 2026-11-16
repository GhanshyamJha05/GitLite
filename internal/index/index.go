package index

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gitlite/internal/object"
	"gitlite/internal/util"
)

const (
	// IndexFile is the default staging area filename within .gitlite.
	IndexFile = "index"
)

var (
	// ErrEntryNotFound is returned when querying an entry that doesn't exist in the index.
	ErrEntryNotFound = errors.New("entry not found in index")
)

// Index represents the staging area holding all staged file entries.
type Index struct {
	// Entries maps relative file path -> Entry.
	Entries map[string]Entry `json:"entries"`
}

// New creates a new empty Index.
func New() *Index {
	return &Index{
		Entries: make(map[string]Entry),
	}
}

// Add stages or updates an entry in the index.
// Automatically normalizes the path to forward slashes.
func (idx *Index) Add(entry Entry) {
	cleanPath := filepath.ToSlash(filepath.Clean(entry.Path))
	cleanPath = strings.TrimPrefix(cleanPath, "./")
	entry.Path = cleanPath
	idx.Entries[cleanPath] = entry
}

// Remove un-stages a file from the index.
func (idx *Index) Remove(relPath string) bool {
	cleanPath := filepath.ToSlash(filepath.Clean(relPath))
	cleanPath = strings.TrimPrefix(cleanPath, "./")
	if _, ok := idx.Entries[cleanPath]; ok {
		delete(idx.Entries, cleanPath)
		return true
	}
	return false
}

// Get retrieves an entry by its relative path.
func (idx *Index) Get(relPath string) (Entry, bool) {
	cleanPath := filepath.ToSlash(filepath.Clean(relPath))
	cleanPath = strings.TrimPrefix(cleanPath, "./")
	entry, ok := idx.Entries[cleanPath]
	return entry, ok
}

// SortedEntries returns all entries sorted lexicographically by relative path.
func (idx *Index) SortedEntries() []Entry {
	list := make([]Entry, 0, len(idx.Entries))
	for _, e := range idx.Entries {
		list = append(list, e)
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].Path < list[j].Path
	})
	return list
}

// Decode parses index bytes from either binary (v2) or legacy JSON format.
func Decode(data []byte) (*Index, error) {
	if len(data) == 0 {
		return New(), nil
	}

	// 1. Try binary format if header magic matches "DIRC"
	if len(data) >= 4 && string(data[:4]) == BinaryIndexMagic {
		return DecodeBinary(data)
	}

	// 2. Fallback to legacy JSON format
	idx := New()
	if err := json.Unmarshal(data, idx); err != nil {
		return nil, fmt.Errorf("decoding index: %w", err)
	}
	if idx.Entries == nil {
		idx.Entries = make(map[string]Entry)
	}
	return idx, nil
}

// Load reads and deserializes the index from the given path (e.g. .gitlite/index).
// Supports both binary checksummed index format (v2) and legacy JSON format.
// If the index file does not exist, an empty Index is returned without error.
func Load(indexPath string) (*Index, error) {
	data, err := os.ReadFile(indexPath)
	if err != nil {
		if os.IsNotExist(err) {
			return New(), nil
		}
		return nil, fmt.Errorf("reading index file: %w", err)
	}

	return Decode(data)
}

// Save atomically writes the index to disk using binary checksummed format with atomic rename.
func (idx *Index) Save(indexPath string) error {
	data, err := idx.EncodeBinary()
	if err != nil {
		return fmt.Errorf("serializing index: %w", err)
	}
	return util.AtomicWriteFile(indexPath, data, 0644)
}

// WriteTree recursively converts the index's staged flat paths into a hierarchical tree object graph,
// writing each Tree object into store and returning the root Tree Hash.
func (idx *Index) WriteTree(store *object.Store) (object.Hash, error) {
	// Build an in-memory directory tree representation
	type dirNode struct {
		subdirs map[string]*dirNode
		files   map[string]object.TreeEntry
	}

	newNode := func() *dirNode {
		return &dirNode{
			subdirs: make(map[string]*dirNode),
			files:   make(map[string]object.TreeEntry),
		}
	}

	root := newNode()

	for _, entry := range idx.SortedEntries() {
		parts := strings.Split(entry.Path, "/")
		curr := root
		for i := 0; i < len(parts)-1; i++ {
			part := parts[i]
			if _, exists := curr.subdirs[part]; !exists {
				curr.subdirs[part] = newNode()
			}
			curr = curr.subdirs[part]
		}
		fileName := parts[len(parts)-1]
		curr.files[fileName] = object.TreeEntry{
			Mode: entry.Mode,
			Name: fileName,
			Hash: entry.Hash,
		}
	}

	// Recursively write trees bottom-up
	var buildTree func(node *dirNode) (object.Hash, error)
	buildTree = func(node *dirNode) (object.Hash, error) {
		var treeEntries []object.TreeEntry

		// Add files
		for _, f := range node.files {
			treeEntries = append(treeEntries, f)
		}

		// Add subdirectories
		for dirName, sub := range node.subdirs {
			subHash, err := buildTree(sub)
			if err != nil {
				return "", err
			}
			treeEntries = append(treeEntries, object.TreeEntry{
				Mode: object.ModeDirectory,
				Name: dirName,
				Hash: subHash,
			})
		}

		tree := object.NewTree(treeEntries)
		return store.Write(tree)
	}

	return buildTree(root)
}
