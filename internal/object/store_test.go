package object_test

import (
	"bytes"
	"compress/zlib"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"gitlite/internal/object"
)

func TestHashValidation(t *testing.T) {
	valid := object.Hash("e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855")
	if err := valid.Validate(); err != nil {
		t.Fatalf("expected valid hash, got %v", err)
	}

	invalidLengths := []object.Hash{
		"abc",
		"e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b85",   // 63 chars
		"e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855a", // 65 chars
		"e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b85G",  // uppercase / non-hex
	}

	for _, inv := range invalidLengths {
		if err := inv.Validate(); err == nil {
			t.Errorf("expected validation failure for %q, got nil", inv)
		}
	}
}

func TestCanonicalEncodeDecode(t *testing.T) {
	content := []byte("hello, systems programming world!\n")
	blob := object.NewBlob(content)

	canonical := object.EncodeCanonical(blob)
	expectedHeader := "blob 34\x00"
	if !bytes.HasPrefix(canonical, []byte(expectedHeader)) {
		t.Fatalf("expected header prefix %q, got %q", expectedHeader, canonical[:len(expectedHeader)])
	}

	objType, payload, err := object.DecodeCanonical(canonical)
	if err != nil {
		t.Fatalf("DecodeCanonical failed: %v", err)
	}

	if objType != object.TypeBlob {
		t.Errorf("expected objType blob, got %s", objType)
	}

	if !bytes.Equal(payload, content) {
		t.Errorf("expected payload %q, got %q", content, payload)
	}
}

func TestStoreWriteAndReadBlob(t *testing.T) {
	tempDir := t.TempDir()
	store := object.NewStore(tempDir)

	data := []byte("The quick brown fox jumps over the lazy dog")
	blob := object.NewBlob(data)

	h, err := store.Write(blob)
	if err != nil {
		t.Fatalf("store.Write failed: %v", err)
	}

	if !store.Has(h) {
		t.Fatalf("store.Has returned false for written object %s", h)
	}

	// Verify 2-level directory layout
	expectedPath := filepath.Join(tempDir, string(h[:2]), string(h[2:]))
	if _, err := os.Stat(expectedPath); err != nil {
		t.Fatalf("expected sharded file at %s: %v", expectedPath, err)
	}

	// Verify retrieval
	retrieved, err := store.ReadBlob(h)
	if err != nil {
		t.Fatalf("store.ReadBlob failed: %v", err)
	}

	if !bytes.Equal(retrieved.Data, data) {
		t.Fatalf("data mismatch: expected %q, got %q", data, retrieved.Data)
	}

	// Test writing identical object returns same hash without error (deduplication)
	h2, err := store.Write(blob)
	if err != nil {
		t.Fatalf("second store.Write failed: %v", err)
	}
	if h != h2 {
		t.Fatalf("expected deduplicated hash %s, got %s", h, h2)
	}
}

func TestStoreCorruptedHash(t *testing.T) {
	tempDir := t.TempDir()
	store := object.NewStore(tempDir)

	data := []byte("unaltered data")
	blob := object.NewBlob(data)

	h, err := store.Write(blob)
	if err != nil {
		t.Fatalf("store.Write failed: %v", err)
	}

	objPath, _ := store.ObjectPath(h)

	// Tamper with the compressed file
	tampered := []byte("corrupted payload")
	var compBuf bytes.Buffer
	zw := zlib.NewWriter(&compBuf)
	_, _ = zw.Write(tampered)
	_ = zw.Close()

	if err := os.WriteFile(objPath, compBuf.Bytes(), 0644); err != nil {
		t.Fatalf("tampering file failed: %v", err)
	}

	// Verification / reading must detect hash mismatch
	_, _, err = store.Read(h)
	if err == nil {
		t.Fatalf("expected hash mismatch error on corrupted object, got nil")
	}
}

func TestConcurrentWrites(t *testing.T) {
	tempDir := t.TempDir()
	store := object.NewStore(tempDir)

	var wg sync.WaitGroup
	data := []byte("concurrent write payload")

	// 20 goroutines attempting to write the exact same object simultaneously
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			b := object.NewBlob(data)
			h, err := store.Write(b)
			if err != nil {
				t.Errorf("concurrent store.Write failed: %v", err)
				return
			}
			if !store.Has(h) {
				t.Errorf("object should exist after write")
			}
		}()
	}

	wg.Wait()
}

func BenchmarkObjectWrite(b *testing.B) {
	tempDir := b.TempDir()
	store := object.NewStore(tempDir)
	data := []byte("benchmarking object write operations in gitlite cas engine")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		blob := object.NewBlob(data)
		if _, err := store.Write(blob); err != nil {
			b.Fatalf("Write failed: %v", err)
		}
	}
}

func BenchmarkObjectRead(b *testing.B) {
	tempDir := b.TempDir()
	store := object.NewStore(tempDir)
	data := []byte("benchmarking object read operations in gitlite cas engine")
	blob := object.NewBlob(data)
	h, err := store.Write(blob)
	if err != nil {
		b.Fatalf("Write failed: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, _, err := store.Read(h); err != nil {
			b.Fatalf("Read failed: %v", err)
		}
	}
}
