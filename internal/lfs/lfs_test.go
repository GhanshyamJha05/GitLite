package lfs_test

import (
	"bytes"
	"io"
	"path/filepath"
	"testing"

	"gitlite/internal/lfs"
)

func TestPointerLifecycle(t *testing.T) {
	fakeOID := "4b9743dd1179d671ec449c28df050f4a434c449c71628d0eeef8bc115b13b708"
	var fakeSize int64 = 10485760 // 10MB

	ptrBytes := lfs.FormatPointer(fakeOID, fakeSize)
	if !lfs.IsPointer(ptrBytes) {
		t.Fatalf("expected IsPointer to be true for formatted pointer")
	}

	parsed, err := lfs.ParsePointer(ptrBytes)
	if err != nil {
		t.Fatalf("ParsePointer failed: %v", err)
	}

	if parsed.OID != fakeOID {
		t.Errorf("expected OID %s, got %s", fakeOID, parsed.OID)
	}
	if parsed.Size != fakeSize {
		t.Errorf("expected Size %d, got %d", fakeSize, parsed.Size)
	}

	// Non-pointer data
	normalText := []byte("Just a normal text file\nhello world\n")
	if lfs.IsPointer(normalText) {
		t.Errorf("IsPointer should be false for normal text")
	}
	_, err = lfs.ParsePointer(normalText)
	if err == nil {
		t.Errorf("expected ParsePointer to fail on normal text")
	}
}

func TestLFSStorage(t *testing.T) {
	tempDir := t.TempDir()
	gitliteDir := filepath.Join(tempDir, ".gitlite")

	content := []byte("Large binary file simulated payload content here!")
	ptr, err := lfs.Store(gitliteDir, bytes.NewReader(content))
	if err != nil {
		t.Fatalf("lfs.Store failed: %v", err)
	}

	if ptr.Size != int64(len(content)) {
		t.Errorf("expected size %d, got %d", len(content), ptr.Size)
	}

	if !lfs.Exists(gitliteDir, ptr.OID) {
		t.Fatalf("expected lfs object %s to exist on disk", ptr.OID)
	}

	rc, err := lfs.Open(gitliteDir, ptr.OID)
	if err != nil {
		t.Fatalf("lfs.Open failed: %v", err)
	}
	defer rc.Close()

	readData, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("reading lfs object: %v", err)
	}

	if !bytes.Equal(readData, content) {
		t.Errorf("retrieved content mismatch: %q vs %q", string(readData), string(content))
	}
}

func TestLFSTrackAndUntrack(t *testing.T) {
	tempDir := t.TempDir()

	// Track patterns
	if err := lfs.Track(tempDir, "*.psd"); err != nil {
		t.Fatalf("Track *.psd failed: %v", err)
	}
	if err := lfs.Track(tempDir, "*.zip"); err != nil {
		t.Fatalf("Track *.zip failed: %v", err)
	}

	// Double-track should be idempotent
	if err := lfs.Track(tempDir, "*.psd"); err != nil {
		t.Fatalf("Duplicate track failed: %v", err)
	}

	tracked, err := lfs.ListTracked(tempDir)
	if err != nil {
		t.Fatalf("ListTracked failed: %v", err)
	}

	if len(tracked) != 2 || tracked[0] != "*.psd" || tracked[1] != "*.zip" {
		t.Errorf("unexpected tracked patterns: %v", tracked)
	}

	// Pattern match
	if !lfs.MatchesPattern("*.psd", "assets/design.psd") {
		t.Errorf("expected *.psd to match assets/design.psd")
	}
	if lfs.MatchesPattern("*.psd", "assets/design.png") {
		t.Errorf("*.psd should not match assets/design.png")
	}

	// Untrack
	if err := lfs.Untrack(tempDir, "*.psd"); err != nil {
		t.Fatalf("Untrack failed: %v", err)
	}

	trackedAfter, err := lfs.ListTracked(tempDir)
	if err != nil {
		t.Fatalf("ListTracked after untrack failed: %v", err)
	}
	if len(trackedAfter) != 1 || trackedAfter[0] != "*.zip" {
		t.Errorf("unexpected tracked patterns after untrack: %v", trackedAfter)
	}
}
