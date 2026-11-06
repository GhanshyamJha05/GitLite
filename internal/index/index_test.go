package index_test

import (
	"path/filepath"
	"testing"

	"gitlite/internal/index"
	"gitlite/internal/object"
)

func TestIndexAddAndRetrieve(t *testing.T) {
	idx := index.New()

	h := object.ComputeHash([]byte("test content"))
	entry := index.Entry{
		Path:        "src/main.go",
		Hash:        h,
		Mode:        object.ModeRegular,
		Size:        100,
		ModTimeNano: 123456789,
	}

	idx.Add(entry)

	retrieved, ok := idx.Get("src/main.go")
	if !ok {
		t.Fatalf("expected entry src/main.go to exist in index")
	}

	if retrieved.Hash != h || retrieved.Size != 100 {
		t.Errorf("entry mismatch: %+v", retrieved)
	}

	// Normalized lookup with leading ./ or backslashes
	retrieved2, ok2 := idx.Get("./src/main.go")
	if !ok2 || retrieved2.Hash != h {
		t.Errorf("path normalization failed on lookup: %+v", retrieved2)
	}

	// Sorted list verification
	idx.Add(index.Entry{Path: "README.md", Hash: h, Mode: object.ModeRegular})
	sorted := idx.SortedEntries()
	if len(sorted) != 2 || sorted[0].Path != "README.md" || sorted[1].Path != "src/main.go" {
		t.Errorf("unexpected sorted order: %v", sorted)
	}

	// Remove entry
	removed := idx.Remove("src/main.go")
	if !removed {
		t.Errorf("expected Remove to return true")
	}
	if _, exists := idx.Get("src/main.go"); exists {
		t.Errorf("expected entry to be removed")
	}
}

func TestIndexSaveAndLoad(t *testing.T) {
	tempDir := t.TempDir()
	indexPath := filepath.Join(tempDir, "index")

	idx := index.New()
	h := object.ComputeHash([]byte("hello index"))
	idx.Add(index.Entry{
		Path:        "docs/spec.md",
		Hash:        h,
		Mode:        object.ModeRegular,
		Size:        50,
		ModTimeNano: 987654321,
	})

	if err := idx.Save(indexPath); err != nil {
		t.Fatalf("idx.Save failed: %v", err)
	}

	loaded, err := index.Load(indexPath)
	if err != nil {
		t.Fatalf("index.Load failed: %v", err)
	}

	entry, ok := loaded.Get("docs/spec.md")
	if !ok {
		t.Fatalf("expected docs/spec.md in loaded index")
	}
	if entry.Hash != h || entry.Size != 50 {
		t.Errorf("loaded entry mismatch: %+v", entry)
	}
}

func TestIndexWriteTree(t *testing.T) {
	tempDir := t.TempDir()
	store := object.NewStore(tempDir)

	idx := index.New()

	h1 := object.ComputeHash([]byte("root file"))
	h2 := object.ComputeHash([]byte("nested file 1"))
	h3 := object.ComputeHash([]byte("nested file 2"))

	idx.Add(index.Entry{Path: "root.txt", Hash: h1, Mode: object.ModeRegular})
	idx.Add(index.Entry{Path: "sub/inner.txt", Hash: h2, Mode: object.ModeRegular})
	idx.Add(index.Entry{Path: "sub/deeper/test.txt", Hash: h3, Mode: object.ModeRegular})

	rootTreeHash, err := idx.WriteTree(store)
	if err != nil {
		t.Fatalf("idx.WriteTree failed: %v", err)
	}

	// Inspect the root tree in the object store
	rootTree, err := store.ReadTree(rootTreeHash)
	if err != nil {
		t.Fatalf("reading root tree %s: %v", rootTreeHash, err)
	}

	// Root tree should have 2 entries: "root.txt" (blob) and "sub" (tree)
	if len(rootTree.Entries) != 2 {
		t.Fatalf("expected 2 entries in root tree, got %d", len(rootTree.Entries))
	}

	if rootTree.Entries[0].Name != "root.txt" || rootTree.Entries[0].Mode != object.ModeRegular {
		t.Errorf("entry 0 mismatch: %+v", rootTree.Entries[0])
	}
	if rootTree.Entries[1].Name != "sub" || rootTree.Entries[1].Mode != object.ModeDirectory {
		t.Errorf("entry 1 mismatch: %+v", rootTree.Entries[1])
	}

	// Inspect the "sub" tree
	subTree, err := store.ReadTree(rootTree.Entries[1].Hash)
	if err != nil {
		t.Fatalf("reading sub tree: %v", err)
	}

	// sub tree should have "deeper" (directory) and "inner.txt" (regular file)
	if len(subTree.Entries) != 2 {
		t.Fatalf("expected 2 entries in sub tree, got %d", len(subTree.Entries))
	}
	if subTree.Entries[0].Name != "deeper" || subTree.Entries[0].Mode != object.ModeDirectory {
		t.Errorf("deeper entry mismatch: %+v", subTree.Entries[0])
	}
	if subTree.Entries[1].Name != "inner.txt" || subTree.Entries[1].Hash != h2 {
		t.Errorf("inner.txt entry mismatch: %+v", subTree.Entries[1])
	}
}
