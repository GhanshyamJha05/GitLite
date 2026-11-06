package object_test

import (
	"testing"
	"time"

	"gitlite/internal/object"
)

func TestCommitSerializationAndParsing(t *testing.T) {
	tempDir := t.TempDir()
	store := object.NewStore(tempDir)

	treeHash := object.ComputeHash([]byte("tree representation"))
	parentHash := object.ComputeHash([]byte("parent commit 1"))

	loc := time.FixedZone("IST", 5*3600+1800)
	now := time.Unix(1710000000, 0).In(loc)

	author := object.Signature{
		Name:  "Ghanshyam Jha",
		Email: "gjha@example.com",
		When:  now,
	}

	commit := object.NewCommit(
		treeHash,
		[]object.Hash{parentHash},
		author,
		author,
		"feat: initial commit for testing\n\nMore details in body.",
	)

	h, err := store.Write(commit)
	if err != nil {
		t.Fatalf("store.Write(commit) failed: %v", err)
	}

	readCommit, err := store.ReadCommit(h)
	if err != nil {
		t.Fatalf("store.ReadCommit failed: %v", err)
	}

	if readCommit.TreeHash != treeHash {
		t.Errorf("tree hash mismatch: expected %s, got %s", treeHash, readCommit.TreeHash)
	}

	if len(readCommit.Parents) != 1 || readCommit.Parents[0] != parentHash {
		t.Errorf("parent hash mismatch: expected [%s], got %v", parentHash, readCommit.Parents)
	}

	if readCommit.Author.Name != author.Name || readCommit.Author.Email != author.Email {
		t.Errorf("author mismatch: expected %+v, got %+v", author, readCommit.Author)
	}

	if readCommit.Author.When.Unix() != now.Unix() {
		t.Errorf("author timestamp mismatch: expected %d, got %d", now.Unix(), readCommit.Author.When.Unix())
	}

	expectedMsg := "feat: initial commit for testing\n\nMore details in body."
	if readCommit.Message != expectedMsg {
		t.Errorf("message mismatch: expected %q, got %q", expectedMsg, readCommit.Message)
	}
}

