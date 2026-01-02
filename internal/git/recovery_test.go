package git

import (
	"os"
	"path/filepath"
	"testing"
)

func TestClient_Recover(t *testing.T) {
	// Create a temp dir
	tmpDir, err := os.MkdirTemp("", "git-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create .git directory
	gitDir := filepath.Join(tmpDir, ".git")
	if err := os.Mkdir(gitDir, 0755); err != nil {
		t.Fatalf("Failed to create .git dir: %v", err)
	}

	// Create a lock file
	lockFile := filepath.Join(gitDir, "index.lock")
	if err := os.WriteFile(lockFile, []byte("lock"), 0644); err != nil {
		t.Fatalf("Failed to create lock file: %v", err)
	}

	// Verify it exists
	if _, err := os.Stat(lockFile); os.IsNotExist(err) {
		t.Fatalf("Lock file should exist")
	}

	// Run Recover
	client := NewClient()
	if err := client.Recover(tmpDir); err != nil {
		t.Fatalf("Recover failed: %v", err)
	}

	// Verify it is gone
	if _, err := os.Stat(lockFile); !os.IsNotExist(err) {
		t.Errorf("Lock file should have been removed")
	}
}
