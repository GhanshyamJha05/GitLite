package refs

import (
	"os"
	"testing"

	"gitlite/internal/object"
)

func TestTagLifecycle(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "gitlite-tag-test-*")
	if err != nil {
		t.Fatalf("temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	mgr := NewManager(tempDir)
	cHash := object.ComputeHash([]byte("fake commit object"))

	// 1. Create tag v1.0.0
	if err := mgr.CreateTag("v1.0.0", cHash); err != nil {
		t.Fatalf("CreateTag failed: %v", err)
	}

	// 2. Reject duplicate
	if err := mgr.CreateTag("v1.0.0", cHash); err == nil {
		t.Fatalf("expected error on duplicate tag creation, got nil")
	}

	// 3. Resolve tag
	resolved, err := mgr.ResolveTag("v1.0.0")
	if err != nil {
		t.Fatalf("ResolveTag failed: %v", err)
	}
	if resolved != cHash {
		t.Errorf("expected hash %s, got %s", cHash, resolved)
	}

	// 4. Create second tag
	if err := mgr.CreateTag("v2.0.0", cHash); err != nil {
		t.Fatalf("create v2.0.0 failed: %v", err)
	}

	// 5. List tags
	tags, err := mgr.ListTags()
	if err != nil {
		t.Fatalf("ListTags failed: %v", err)
	}
	if len(tags) != 2 {
		t.Fatalf("expected 2 tags, got %d", len(tags))
	}
	if tags[0].Name != "v1.0.0" || tags[1].Name != "v2.0.0" {
		t.Errorf("unexpected tags: %+v", tags)
	}

	// 6. Delete tag
	if err := mgr.DeleteTag("v1.0.0"); err != nil {
		t.Fatalf("DeleteTag failed: %v", err)
	}

	_, err = mgr.ResolveTag("v1.0.0")
	if err == nil {
		t.Fatalf("expected error resolving deleted tag, got nil")
	}
}
