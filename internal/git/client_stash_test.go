package git

import (
	"os"
	"path/filepath"
	"testing"
)

func TestClient_StashPopEmpty(t *testing.T) {
	localDir, _ := setupTestRepo(t)
	defer os.RemoveAll(localDir)

	c := NewClient()

	// Initial commit
	os.WriteFile(filepath.Join(localDir, "f1"), []byte("v1"), 0644)
	c.Commit(localDir, "init")

	// Verify no stashes exist
	if err := c.Stash(localDir); err != nil {
		t.Fatalf("Stash failed: %v", err)
	}

	// StashPop should succeed (no-op)
	if err := c.StashPop(localDir); err != nil {
		t.Errorf("StashPop failed on empty stash: %v", err)
	}
}

func TestClient_StashPopNonEmpty(t *testing.T) {
	localDir, _ := setupTestRepo(t)
	defer os.RemoveAll(localDir)

	c := NewClient()

	// Initial commit
	os.WriteFile(filepath.Join(localDir, "f1"), []byte("v1"), 0644)
	c.Commit(localDir, "init")

	// Modify file
	os.WriteFile(filepath.Join(localDir, "f1"), []byte("v2"), 0644)

	// Stash changes
	if err := c.Stash(localDir); err != nil {
		t.Fatalf("Stash failed: %v", err)
	}

	// Verify content reverted
	content, _ := os.ReadFile(filepath.Join(localDir, "f1"))
	if string(content) != "v1" {
		t.Fatalf("Stash failed to revert changes")
	}

	// StashPop
	if err := c.StashPop(localDir); err != nil {
		t.Errorf("StashPop failed on non-empty stash: %v", err)
	}

	// Verify content restored
	content, _ = os.ReadFile(filepath.Join(localDir, "f1"))
	if string(content) != "v2" {
		t.Errorf("StashPop failed to restore changes")
	}
}
