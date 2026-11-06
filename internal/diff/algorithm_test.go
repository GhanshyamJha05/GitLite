package diff_test

import (
	"strings"
	"testing"

	"gitlite/internal/diff"
)

func TestComputeLCS(t *testing.T) {
	a := "apple\nbanana\ncherry\n"
	b := "apple\nblueberry\ncherry\ndate\n"

	edits := diff.ComputeLCS(a, b)

	expected := []struct {
		op   diff.OpType
		text string
	}{
		{diff.OpEqual, "apple"},
		{diff.OpDelete, "banana"},
		{diff.OpInsert, "blueberry"},
		{diff.OpEqual, "cherry"},
		{diff.OpInsert, "date"},
	}

	if len(edits) != len(expected) {
		t.Fatalf("expected %d edits, got %d: %+v", len(expected), len(edits), edits)
	}

	for i, exp := range expected {
		if edits[i].Type != exp.op || edits[i].Text != exp.text {
			t.Errorf("edit %d mismatch: expected {%v, %s}, got {%v, %s}", i, exp.op, exp.text, edits[i].Type, edits[i].Text)
		}
	}
}

func TestUnifiedDiff(t *testing.T) {
	a := "line 1\nline 2\n"
	b := "line 1\nline 2 modified\nline 3 added\n"

	udiff := diff.UnifiedDiff("file.txt", "file.txt", a, b)
	if !strings.Contains(udiff, "--- a/file.txt") || !strings.Contains(udiff, "+++ b/file.txt") {
		t.Errorf("missing headers in unified diff: %s", udiff)
	}

	if !strings.Contains(udiff, "-line 2") || !strings.Contains(udiff, "+line 2 modified") || !strings.Contains(udiff, "+line 3 added") {
		t.Errorf("missing edit markers in diff: %s", udiff)
	}

	// Unchanged text should produce empty diff
	identical := diff.UnifiedDiff("file.txt", "file.txt", a, a)
	if identical != "" {
		t.Errorf("expected empty diff for identical inputs, got: %s", identical)
	}
}
