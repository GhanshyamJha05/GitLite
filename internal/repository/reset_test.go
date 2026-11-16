package repository

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"gitlite/internal/object"
)

func TestResetModes(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "gitlite-reset-test-*")
	if err != nil {
		t.Fatalf("temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	repo, err := Init(tempDir)
	if err != nil {
		t.Fatalf("init repo: %v", err)
	}

	sig := object.Signature{Name: "Tester", Email: "test@example.com", When: time.Now()}

	// Commit 1: file1.txt
	f1 := filepath.Join(tempDir, "file1.txt")
	_ = os.WriteFile(f1, []byte("version 1\n"), 0644)
	_ = repo.Add(f1)
	c1, err := repo.Commit("commit 1", sig, sig)
	if err != nil {
		t.Fatalf("c1 failed: %v", err)
	}

	// Commit 2: file2.txt & modified file1.txt
	f2 := filepath.Join(tempDir, "file2.txt")
	_ = os.WriteFile(f1, []byte("version 2\n"), 0644)
	_ = os.WriteFile(f2, []byte("file 2 content\n"), 0644)
	_ = repo.Add(f1)
	_ = repo.Add(f2)
	c2, err := repo.Commit("commit 2", sig, sig)
	if err != nil {
		t.Fatalf("c2 failed: %v", err)
	}

	// 1. Test ResetSoft to c1
	if err := repo.Reset(c1, ResetSoft); err != nil {
		t.Fatalf("ResetSoft failed: %v", err)
	}

	// HEAD should now be at c1
	headH, _, _ := repo.Refs().ResolveHead()
	if headH != c1 {
		t.Errorf("expected HEAD %s, got %s", c1, headH)
	}

	// Index should still have c2's entries
	idx, _ := repo.LoadIndex()
	if _, staged := idx.Get("file2.txt"); !staged {
		t.Errorf("expected file2.txt to still be staged after soft reset")
	}

	// Move back to c2 with files staged
	_ = repo.Add(f1)
	_ = repo.Add(f2)
	_ = repo.Reset(c2, ResetHard)

	// 2. Test ResetMixed to c1
	if err := repo.Reset(c1, ResetMixed); err != nil {
		t.Fatalf("ResetMixed failed: %v", err)
	}

	headH, _, _ = repo.Refs().ResolveHead()
	if headH != c1 {
		t.Errorf("expected HEAD %s, got %s", c1, headH)
	}

	idx, _ = repo.LoadIndex()
	if _, staged := idx.Get("file2.txt"); staged {
		t.Errorf("expected file2.txt to NOT be in index after mixed reset")
	}
	// Working tree file2.txt should still exist
	if _, err := os.Stat(f2); err != nil {
		t.Errorf("expected file2.txt to remain in work tree after mixed reset")
	}

	// Move back to c2 with c2 tree in index before hard reset
	_ = repo.Reset(c2, ResetHard)

	// 3. Test ResetHard to c1
	if err := repo.Reset(c1, ResetHard); err != nil {
		t.Fatalf("ResetHard failed: %v", err)
	}

	// file2.txt should be deleted from work tree
	if _, err := os.Stat(f2); !os.IsNotExist(err) {
		t.Errorf("expected file2.txt to be removed after hard reset")
	}
	f1Data, _ := os.ReadFile(f1)
	if string(f1Data) != "version 1\n" {
		t.Errorf("expected file1.txt restored to version 1, got %q", string(f1Data))
	}
}
