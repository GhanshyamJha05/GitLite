package ignore

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIgnoreBasicPatterns(t *testing.T) {
	m := NewMatcher()
	m.AddLine("*.log", "")
	m.AddLine("build/", "")
	m.AddLine("temp.txt", "")
	m.AddLine("!important.log", "")

	tests := []struct {
		path     string
		isDir    bool
		expected bool
	}{
		{"app.log", false, true},
		{"nested/app.log", false, true},
		{"important.log", false, false},
		{"nested/important.log", false, false},
		{"temp.txt", false, true},
		{"other.txt", false, false},
		{"build", true, true},
		{"build/output.bin", false, true},
		{"build", false, false}, // build/ with dirOnly should not match a file named build
	}

	for _, tt := range tests {
		got := m.Matches(tt.path, tt.isDir)
		if got != tt.expected {
			t.Errorf("Matches(%q, isDir=%v): expected %v, got %v", tt.path, tt.isDir, tt.expected, got)
		}
	}
}

func TestIgnoreLoadFile(t *testing.T) {
	tempDir := t.TempDir()
	ignorePath := filepath.Join(tempDir, ".gitliteignore")
	content := []byte("# Comment\n*.tmp\nnode_modules/\n!keep.tmp\n")
	if err := os.WriteFile(ignorePath, content, 0644); err != nil {
		t.Fatal(err)
	}

	m := NewMatcher()
	if err := m.LoadFile(ignorePath, ""); err != nil {
		t.Fatalf("LoadFile failed: %v", err)
	}

	if !m.Matches("cache.tmp", false) {
		t.Errorf("expected cache.tmp to be ignored")
	}
	if m.Matches("keep.tmp", false) {
		t.Errorf("expected keep.tmp to not be ignored")
	}
	if !m.Matches("node_modules", true) {
		t.Errorf("expected node_modules dir to be ignored")
	}
}

