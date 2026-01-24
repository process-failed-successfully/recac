package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSnapshotCreate(t *testing.T) {
	// Setup temp workspace
	tmpDir := t.TempDir()
	cwd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(cwd)

	// Create dummy agent state
	os.WriteFile(filepath.Join(tmpDir, ".agent_state.json"), []byte("{}"), 0644)

	// Mock Git Client
	mockGit := &MockGitClient{
		RepoExistsFunc: func(repoPath string) bool {
			return true
		},
		TagFunc: func(repoPath, version string) error {
			assert.Equal(t, "snapshot/test-snap", version)
			return nil
		},
	}
	// Inject Factory
	oldFactory := gitClientFactory
	gitClientFactory = func() IGitClient {
		return mockGit
	}
	defer func() { gitClientFactory = oldFactory }()

	// Execute
	output, err := executeCommand(rootCmd, "snapshot", "create", "test-snap")
	require.NoError(t, err, "Output: %s", output)

	// Verify Files
	snapshotDir := filepath.Join(tmpDir, ".recac", "snapshots", "test-snap")
	assert.FileExists(t, filepath.Join(snapshotDir, ".agent_state.json"))
}

func TestSnapshotList(t *testing.T) {
	tmpDir := t.TempDir()
	cwd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(cwd)

	// Create dummy snapshot
	snapDir := filepath.Join(tmpDir, ".recac", "snapshots", "snap1")
	os.MkdirAll(snapDir, 0755)
	// Set mtime
	now := time.Now()
	os.Chtimes(snapDir, now, now)

	// Execute
	output, err := executeCommand(rootCmd, "snapshot", "list")
	require.NoError(t, err)
	assert.Contains(t, output, "snap1")
	assert.Contains(t, output, "snapshot/snap1")
}

func TestSnapshotRestore(t *testing.T) {
	tmpDir := t.TempDir()
	cwd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(cwd)

	// Create snapshot with state
	snapDir := filepath.Join(tmpDir, ".recac", "snapshots", "restore-me")
	os.MkdirAll(snapDir, 0755)
	os.WriteFile(filepath.Join(snapDir, ".agent_state.json"), []byte(`{"foo":"bar"}`), 0644)

	// Mock Git
	mockGit := &MockGitClient{
		CheckoutFunc: func(repoPath, commitOrBranch string) error {
			assert.Equal(t, "snapshot/restore-me", commitOrBranch)
			return nil
		},
	}
	oldFactory := gitClientFactory
	gitClientFactory = func() IGitClient {
		return mockGit
	}
	defer func() { gitClientFactory = oldFactory }()

	// Execute
	output, err := executeCommand(rootCmd, "snapshot", "restore", "restore-me")
	require.NoError(t, err, "Output: %s", output)

	// Verify restored file
	content, err := os.ReadFile(filepath.Join(tmpDir, ".agent_state.json"))
	require.NoError(t, err)
	assert.Equal(t, `{"foo":"bar"}`, string(content))
}

func TestSnapshotDelete(t *testing.T) {
	tmpDir := t.TempDir()
	cwd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(cwd)

	// Create snapshot
	snapDir := filepath.Join(tmpDir, ".recac", "snapshots", "delete-me")
	os.MkdirAll(snapDir, 0755)

	// Mock Git
	mockGit := &MockGitClient{
		DeleteTagFunc: func(repoPath, version string) error {
			assert.Equal(t, "snapshot/delete-me", version)
			return nil
		},
	}
	oldFactory := gitClientFactory
	gitClientFactory = func() IGitClient {
		return mockGit
	}
	defer func() { gitClientFactory = oldFactory }()

	// Execute
	output, err := executeCommand(rootCmd, "snapshot", "delete", "delete-me")
	require.NoError(t, err, "Output: %s", output)

	// Verify deleted
	assert.NoDirExists(t, snapDir)
}
