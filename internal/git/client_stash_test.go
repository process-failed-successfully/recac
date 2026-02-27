package git

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestClient_Run(t *testing.T) {
	localDir, _ := setupTestRepo(t)
	defer os.RemoveAll(localDir)

	c := NewClient()

	// Test Run with a simple command
	out, err := c.Run(localDir, "status")
	if err != nil {
		t.Errorf("Run failed: %v", err)
	}
	if !strings.Contains(out, "On branch") {
		t.Errorf("Expected status output, got: %s", out)
	}

	// Test Run with error
	_, err = c.Run(localDir, "invalid-command")
	if err == nil {
		t.Error("Run should fail with invalid command")
	}
}

func TestClient_StashOperations(t *testing.T) {
	localDir, _ := setupTestRepo(t)
	defer os.RemoveAll(localDir)

	c := NewClient()

	// Initial commit
	os.WriteFile(filepath.Join(localDir, "f1"), []byte("v1"), 0644)
	c.Commit(localDir, "init")

	// 1. Test StashPush
	// Modify file
	os.WriteFile(filepath.Join(localDir, "f1"), []byte("v2"), 0644)
	if err := c.StashPush(localDir, "stash message"); err != nil {
		t.Errorf("StashPush failed: %v", err)
	}

	// Verify content reverted
	content, _ := os.ReadFile(filepath.Join(localDir, "f1"))
	if string(content) != "v1" {
		t.Errorf("StashPush didn't revert changes. Got %s", string(content))
	}

	// 2. Test StashList
	list, err := c.StashList(localDir)
	if err != nil {
		t.Errorf("StashList failed: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("Expected 1 stash, got %d", len(list))
	}
	if !strings.Contains(list[0], "stash message") {
		t.Errorf("Expected stash message, got %s", list[0])
	}

	// 3. Test StashShow
	// The stash should show the diff (v1 -> v2)
	show, err := c.StashShow(localDir, "stash@{0}")
	if err != nil {
		t.Errorf("StashShow failed: %v", err)
	}
	if !strings.Contains(show, "v2") {
		t.Errorf("StashShow expected to contain 'v2', got: %s", show)
	}

	// 4. Test StashApply
	if err := c.StashApply(localDir, "stash@{0}"); err != nil {
		t.Errorf("StashApply failed: %v", err)
	}
	// Verify content restored
	content, _ = os.ReadFile(filepath.Join(localDir, "f1"))
	if string(content) != "v2" {
		t.Errorf("StashApply didn't restore changes. Got %s", string(content))
	}

	// 5. Test StashDrop
	// Reset changes first so we can drop cleanly if needed (though drop just removes the stash entry)
	// Let's create another stash to test drop specifically
	// Revert to v1
	c.ResetHard(localDir, "origin", "master") // Assuming master is default from setupTestRepo
	os.WriteFile(filepath.Join(localDir, "f1"), []byte("v3"), 0644)
	c.StashPush(localDir, "stash 2")

	list, _ = c.StashList(localDir)
	if len(list) != 2 { // We kept the previous one because we used Apply, not Pop
		t.Errorf("Expected 2 stashes, got %d", len(list))
	}

	if err := c.StashDrop(localDir, "stash@{0}"); err != nil {
		t.Errorf("StashDrop failed: %v", err)
	}

	list, _ = c.StashList(localDir)
	if len(list) != 1 {
		t.Errorf("Expected 1 stash after drop, got %d", len(list))
	}

	// 6. Test StashClear
	if err := c.StashClear(localDir); err != nil {
		t.Errorf("StashClear failed: %v", err)
	}
	list, _ = c.StashList(localDir)
	if len(list) != 0 {
		t.Errorf("Expected 0 stashes after clear, got %d", len(list))
	}
}
