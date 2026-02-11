package workflow

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRunWorkflow_DirtyCheck(t *testing.T) {
	// Setup git repo
	tmpDir := t.TempDir()
	exec.Command("git", "init", tmpDir).Run()
	exec.Command("git", "-C", tmpDir, "config", "user.email", "test@test.com").Run()
	exec.Command("git", "-C", tmpDir, "config", "user.name", "Test").Run()

	// Create a file and commit it
	os.WriteFile(filepath.Join(tmpDir, "file.txt"), []byte("content"), 0644)
	exec.Command("git", "-C", tmpDir, "add", ".").Run()
	exec.Command("git", "-C", tmpDir, "commit", "-m", "init").Run()

	// Modify file (dirty)
	os.WriteFile(filepath.Join(tmpDir, "file.txt"), []byte("changed"), 0644)

	cfg := SessionConfig{
		ProjectPath: tmpDir,
		SessionName: "dirty-test",
		AllowDirty:  false,
		IsMock:      false,
	}

	err := RunWorkflow(context.Background(), cfg)
	if err == nil {
		t.Error("Expected error for dirty repo")
	}
	assert.Contains(t, err.Error(), "uncommitted changes detected")
}

func TestRunWorkflow_DirtyCheck_Allowed(t *testing.T) {
	// Setup git repo
	tmpDir := t.TempDir()
	exec.Command("git", "init", tmpDir).Run()
	exec.Command("git", "-C", tmpDir, "config", "user.email", "test@test.com").Run()
	exec.Command("git", "-C", tmpDir, "config", "user.name", "Test").Run()

	// Modify file (dirty)
	os.WriteFile(filepath.Join(tmpDir, "file.txt"), []byte("changed"), 0644)

	// Mock NewSessionFunc to avoid actual session start
	originalNewSessionFunc := NewSessionFunc
	defer func() { NewSessionFunc = originalNewSessionFunc }()
	NewSessionFunc = nil // This will panic if called, proving we passed the check but failed later

	cfg := SessionConfig{
		ProjectPath: tmpDir,
		SessionName: "dirty-allowed-test",
		AllowDirty:  true,
		IsMock:      false,
	}

	// We expect a panic because we set NewSessionFunc to nil, or error if other things fail first
	defer func() {
		if r := recover(); r != nil {
			// Panic expected means we passed dirty check
		}
	}()

	err := RunWorkflow(context.Background(), cfg)
	// If it returns error (e.g. Docker fail), that's fine too, as long as it's not "uncommitted changes"
	if err != nil {
		assert.NotContains(t, err.Error(), "uncommitted changes detected")
	}
}
