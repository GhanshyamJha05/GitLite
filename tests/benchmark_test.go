package tests

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"gitlite/internal/diff"
	"gitlite/internal/index"
	"gitlite/internal/object"
	"gitlite/internal/repository"
)

func BenchmarkObjectStoreWrite(b *testing.B) {
	tempDir := b.TempDir()
	store := object.NewStore(tempDir)
	data := []byte("benchmarking raw blob storage write performance in gitlite")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		blob := object.NewBlob(data)
		if _, err := store.Write(blob); err != nil {
			b.Fatalf("Write failed: %v", err)
		}
	}
}

func BenchmarkObjectStoreRead(b *testing.B) {
	tempDir := b.TempDir()
	store := object.NewStore(tempDir)
	data := []byte("benchmarking raw blob storage read and verification performance in gitlite")
	blob := object.NewBlob(data)
	h, err := store.Write(blob)
	if err != nil {
		b.Fatalf("Write failed: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := store.ReadBlob(h); err != nil {
			b.Fatalf("ReadBlob failed: %v", err)
		}
	}
}

func BenchmarkLCSDiff(b *testing.B) {
	a := "line 1\nline 2\nline 3\nline 4\nline 5\nline 6\nline 7\nline 8\nline 9\nline 10\n"
	c := "line 1\nline 2 mod\nline 3\nline 4\nline 5 mod\nline 6\nline 7\nline 8\nline 9\nline 10 mod\n"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = diff.ComputeLCS(a, c)
	}
}

func BenchmarkHierarchicalTreeGeneration(b *testing.B) {
	tempDir := b.TempDir()
	store := object.NewStore(tempDir)

	idx := index.New()
	dummyHash := object.ComputeHash([]byte("benchmark content"))

	// Build a staged index with 100 nested files
	for i := 0; i < 100; i++ {
		path := fmt.Sprintf("pkg/sub%d/deep/file_%d.go", i%5, i)
		idx.Add(index.Entry{
			Path: path,
			Hash: dummyHash,
			Mode: object.ModeRegular,
		})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := idx.WriteTree(store); err != nil {
			b.Fatalf("WriteTree failed: %v", err)
		}
	}
}

func BenchmarkRepositoryStatus(b *testing.B) {
	tempDir := b.TempDir()
	repo, err := repository.Init(tempDir)
	if err != nil {
		b.Fatalf("Init failed: %v", err)
	}

	// Create 50 files in working directory
	for i := 0; i < 50; i++ {
		p := filepath.Join(tempDir, fmt.Sprintf("file_%d.txt", i))
		_ = os.WriteFile(p, []byte("content"), 0644)
		_ = repo.Add(p)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := repo.Status(); err != nil {
			b.Fatalf("Status failed: %v", err)
		}
	}
}
