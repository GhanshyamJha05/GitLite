package object_test

import (
	"bytes"
	"testing"

	"gitlite/internal/object"
)

func TestTreeSerializationAndParsing(t *testing.T) {
	tempDir := t.TempDir()
	store := object.NewStore(tempDir)

	blob1Hash := object.ComputeHash([]byte("file 1 content"))
	blob2Hash := object.ComputeHash([]byte("file 2 content"))

	entries := []object.TreeEntry{
		{
			Mode: object.ModeRegular,
			Name: "zebra.txt",
			Hash: blob2Hash,
		},
		{
			Mode: object.ModeRegular,
			Name: "apple.txt",
			Hash: blob1Hash,
		},
	}

	tree := object.NewTree(entries)
	// NewTree should sort entries alphabetically: apple.txt then zebra.txt
	if tree.Entries[0].Name != "apple.txt" || tree.Entries[1].Name != "zebra.txt" {
		t.Fatalf("tree entries not sorted correctly: %v", tree.Entries)
	}

	h, err := store.Write(tree)
	if err != nil {
		t.Fatalf("store.Write(tree) failed: %v", err)
	}

	readTree, err := store.ReadTree(h)
	if err != nil {
		t.Fatalf("store.ReadTree failed: %v", err)
	}

	if len(readTree.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(readTree.Entries))
	}

	if readTree.Entries[0].Name != "apple.txt" || readTree.Entries[0].Hash != blob1Hash {
		t.Errorf("entry 0 mismatch: %+v", readTree.Entries[0])
	}
	if readTree.Entries[1].Name != "zebra.txt" || readTree.Entries[1].Hash != blob2Hash {
		t.Errorf("entry 1 mismatch: %+v", readTree.Entries[1])
	}

	// Canonical payload match
	if !bytes.Equal(tree.SerializeContent(), readTree.SerializeContent()) {
		t.Errorf("serialized payloads do not match")
	}
}
