package merge_test

import (
	"strings"
	"testing"

	"gitlite/internal/merge"
	"gitlite/internal/object"
)

type mockCommitStore struct {
	commits map[object.Hash]*object.Commit
}

func (m *mockCommitStore) ReadCommit(h object.Hash) (*object.Commit, error) {
	c, ok := m.commits[h]
	if !ok {
		return nil, object.ErrObjectNotFound
	}
	return c, nil
}

func TestFindCommonAncestor(t *testing.T) {
	// Build a linear graph: C1 -> C2 -> C3
	// And a branch: C2 -> C4
	// Common ancestor of C3 and C4 should be C2
	c1 := object.ComputeHash([]byte("c1"))
	c2 := object.ComputeHash([]byte("c2"))
	c3 := object.ComputeHash([]byte("c3"))
	c4 := object.ComputeHash([]byte("c4"))

	store := &mockCommitStore{
		commits: map[object.Hash]*object.Commit{
			c1: {Parents: nil},
			c2: {Parents: []object.Hash{c1}},
			c3: {Parents: []object.Hash{c2}},
			c4: {Parents: []object.Hash{c2}},
		},
	}

	lca, err := merge.FindCommonAncestor(store, c3, c4)
	if err != nil {
		t.Fatalf("FindCommonAncestor failed: %v", err)
	}
	if lca != c2 {
		t.Errorf("expected LCA %s, got %s", c2, lca)
	}
}

func TestThreeWayMergeContent(t *testing.T) {
	base := "alpha\nbeta\ngamma\n"

	// Non-conflicting: only incoming changed
	incoming := "alpha\nbeta\ngamma\ndelta\n"
	res := merge.ThreeWayMergeContent(base, base, incoming)
	if res.Conflict {
		t.Errorf("unexpected conflict in clean merge")
	}
	if res.Content != incoming {
		t.Errorf("expected clean incoming content, got: %s", res.Content)
	}

	// Conflicting: both changed differently
	curMod := "alpha\nbeta modified in cur\ngamma\n"
	incMod := "alpha\nbeta modified in inc\ngamma\n"
	confRes := merge.ThreeWayMergeContent(base, curMod, incMod)
	if !confRes.Conflict {
		t.Errorf("expected merge conflict for divergent modifications")
	}
	if !strings.Contains(confRes.Content, "<<<<<<< CURRENT") || !strings.Contains(confRes.Content, "=======") || !strings.Contains(confRes.Content, ">>>>>>> INCOMING") {
		t.Errorf("missing conflict markers in merged output:\n%s", confRes.Content)
	}

	// Non-overlapping changes on different lines: both should be cleanly merged without conflict
	baseText := "header\nsection 1\nmiddle\nsection 2\nfooter\n"
	curText := "header\nsection 1 updated by current\nmiddle\nsection 2\nfooter\n"
	incText := "header\nsection 1\nmiddle\nsection 2 updated by incoming\nfooter\n"
	expectedMerged := "header\nsection 1 updated by current\nmiddle\nsection 2 updated by incoming\nfooter\n"

	cleanNonOverlappingRes := merge.ThreeWayMergeContent(baseText, curText, incText)
	if cleanNonOverlappingRes.Conflict {
		t.Errorf("expected clean 3-way merge for non-overlapping edits, but got conflict:\n%s", cleanNonOverlappingRes.Content)
	}
	if cleanNonOverlappingRes.Content != expectedMerged {
		t.Errorf("expected merged content:\n%q\ngot:\n%q", expectedMerged, cleanNonOverlappingRes.Content)
	}
}
