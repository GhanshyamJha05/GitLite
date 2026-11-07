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

	// Multiple hunks separated by > 2*context lines (default contextLines=3)
	textA := "L1\nL2\nL3\nL4\nL5\nL6\nL7\nL8\nL9\nL10\nL11\nL12\nL13\nL14\nL15\nL16\nL17\nL18\nL19\nL20\n"
	textB := "L1 mod\nL2\nL3\nL4\nL5\nL6\nL7\nL8\nL9\nL10\nL11\nL12\nL13\nL14\nL15\nL16\nL17\nL18\nL19\nL20 mod\n"

	multiHunkDiff := diff.UnifiedDiff("file.txt", "file.txt", textA, textB)
	// Should have two separate @@ hunk headers
	hunkCount := strings.Count(multiHunkDiff, "@@ ")
	if hunkCount != 2 {
		t.Errorf("expected 2 distinct hunks, got %d:\n%s", hunkCount, multiHunkDiff)
	}
	if !strings.Contains(multiHunkDiff, "@@ -1,4 +1,4 @@") {
		t.Errorf("expected first hunk to target line 1, got:\n%s", multiHunkDiff)
	}
	if !strings.Contains(multiHunkDiff, "-L1\n+L1 mod") || !strings.Contains(multiHunkDiff, "-L20\n+L20 mod") {
		t.Errorf("expected hunk modifications for both L1 and L20:\n%s", multiHunkDiff)
	}
}
