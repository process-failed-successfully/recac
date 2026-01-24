package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"recac/internal/git"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSnapshotCmd(t *testing.T) {
	// Override factories
	origGitFactory := gitClientFactory
	origSessionManagerFactory := sessionManagerFactory
	defer func() {
		gitClientFactory = origGitFactory
		sessionManagerFactory = origSessionManagerFactory
	}()

	// Use real git client
	gitClientFactory = func() IGitClient {
		return git.NewClient()
	}

	t.Run("Save and List and Restore", func(t *testing.T) {
		// 1. Setup workspace with Git
		tempDir, err := os.MkdirTemp("", "recac-snapshot-test")
		require.NoError(t, err)
		defer os.RemoveAll(tempDir)

		setupTestGitRepo(t, tempDir)

		// Create state files
		stateFile := filepath.Join(tempDir, ".agent_state.json")
		dbFile := filepath.Join(tempDir, ".recac.db")
		require.NoError(t, os.WriteFile(stateFile, []byte(`{"state":"initial"}`), 0644))
		require.NoError(t, os.WriteFile(dbFile, []byte(`initial-db`), 0644))

		// 2. Mock Session Manager to resolve workspace
		mockSM := NewMockSessionManager()
		sessionManagerFactory = func() (ISessionManager, error) { return mockSM, nil }
		// Create a mock session that uses this workspace
		mockSM.StartSession("test-session", "goal", []string{}, tempDir)

		// 3. Save Snapshot
		// Note: We need to use root command to propagate flags
		output, err := executeCommand(rootCmd, "snapshot", "save", "snap1", "--session", "test-session", "--description", "my snapshot")
		require.NoError(t, err)
		assert.Contains(t, output, "Snapshot 'snap1' saved successfully")

		// Verify files
		snapDir := filepath.Join(tempDir, ".recac", "snapshots", "snap1")
		assert.FileExists(t, filepath.Join(snapDir, "meta.json"))
		assert.FileExists(t, filepath.Join(snapDir, ".agent_state.json"))
		assert.FileExists(t, filepath.Join(snapDir, ".recac.db"))

		// Verify Git Tag
		tagOut, _ := exec.Command("git", "-C", tempDir, "tag", "-l", "snapshot/snap1").Output()
		assert.Contains(t, string(tagOut), "snapshot/snap1")

		// 4. Modify state
		require.NoError(t, os.WriteFile(stateFile, []byte(`{"state":"modified"}`), 0644))
		require.NoError(t, os.WriteFile(dbFile, []byte(`modified-db`), 0644))

		// 5. List Snapshots
		output, err = executeCommand(rootCmd, "snapshot", "list", "--session", "test-session")
		require.NoError(t, err)
		assert.Contains(t, output, "snap1")
		assert.Contains(t, output, "my snapshot")

		// 6. Restore Snapshot
		output, err = executeCommand(rootCmd, "snapshot", "restore", "snap1", "--session", "test-session")
		require.NoError(t, err)
		assert.Contains(t, output, "Snapshot 'snap1' restored successfully")

		// Verify content restored
		content, _ := os.ReadFile(stateFile)
		assert.Equal(t, `{"state":"initial"}`, string(content))
		content, _ = os.ReadFile(dbFile)
		assert.Equal(t, `initial-db`, string(content))
	})

	t.Run("Delete Snapshot", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "recac-snapshot-delete-test")
		require.NoError(t, err)
		defer os.RemoveAll(tempDir)
		setupTestGitRepo(t, tempDir)

		mockSM := NewMockSessionManager()
		sessionManagerFactory = func() (ISessionManager, error) { return mockSM, nil }
		mockSM.StartSession("test-session", "goal", []string{}, tempDir)

		// Create snapshot
		executeCommand(rootCmd, "snapshot", "save", "snap-del", "--session", "test-session")

		// Delete
		output, err := executeCommand(rootCmd, "snapshot", "delete", "snap-del", "--session", "test-session")
		require.NoError(t, err)
		assert.Contains(t, output, "deleted successfully")

		// Verify dir gone
		snapDir := filepath.Join(tempDir, ".recac", "snapshots", "snap-del")
		assert.NoDirExists(t, snapDir)

		// Verify tag gone
		tagOut, _ := exec.Command("git", "-C", tempDir, "tag", "-l", "snapshot/snap-del").Output()
		assert.NotContains(t, string(tagOut), "snapshot/snap-del")
	})
}

func setupTestGitRepo(t *testing.T, dir string) {
	runTestGit(t, dir, "init")
	runTestGit(t, dir, "config", "user.email", "you@example.com")
	runTestGit(t, dir, "config", "user.name", "Your Name")
	// Commit something so HEAD exists
	os.WriteFile(filepath.Join(dir, "README"), []byte("init"), 0644)
	runTestGit(t, dir, "add", ".")
	runTestGit(t, dir, "commit", "-m", "initial")
}

func runTestGit(t *testing.T, dir string, args ...string) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	err := cmd.Run()
	require.NoError(t, err, "git %v failed", args)
}
