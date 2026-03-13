package main

import (
	"bytes"
	"os"
	"path/filepath"
	"recac/internal/runner"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveWorkspace(t *testing.T) {
	tmpDir := t.TempDir()

	mockSM := NewMockSessionManager()
	mockSM.Sessions["test-session"] = &runner.SessionState{Workspace: tmpDir}

	origSMFactory := sessionManagerFactory
	sessionManagerFactory = func() (ISessionManager, error) { return mockSM, nil }
	defer func() { sessionManagerFactory = origSMFactory }()

	origGitFactory := gitClientFactory
	mockGit := &MockGitClient{
		RepoExistsFunc: func(path string) bool { return true },
	}
	gitClientFactory = func() IGitClient { return mockGit }
	defer func() { gitClientFactory = origGitFactory }()

	cmd := &cobra.Command{}

	// Test 1: By session name
	ws, err := resolveWorkspace(cmd, []string{"test-session"})
	require.NoError(t, err)
	assert.Equal(t, tmpDir, ws)

	// Test 2: Current directory
	cwd, _ := os.Getwd()
	ws, err = resolveWorkspace(cmd, []string{})
	require.NoError(t, err)
	assert.Equal(t, cwd, ws)
}

func TestRollbackCmd(t *testing.T) {
	tmpDir := t.TempDir()

	// Create state files to test deletion
	os.WriteFile(filepath.Join(tmpDir, ".agent_state.json"), []byte("{}"), 0644)
	os.WriteFile(filepath.Join(tmpDir, ".recac.db"), []byte("{}"), 0644)

	mockSM := NewMockSessionManager()
	origSMFactory := sessionManagerFactory
	sessionManagerFactory = func() (ISessionManager, error) { return mockSM, nil }
	defer func() { sessionManagerFactory = origSMFactory }()

	checkoutCalled := false
	checkoutNewBranchCalled := false

	origGitFactory := gitClientFactory
	mockGit := &MockGitClient{
		RepoExistsFunc: func(path string) bool { return true },
		CurrentBranchFunc: func(path string) (string, error) {
			return "main", nil
		},
		CheckoutFunc: func(repoPath, branch string) error {
			checkoutCalled = true
			assert.Equal(t, "abc1234", branch)
			return nil
		},
		CheckoutNewBranchFunc: func(repoPath, branch string) error {
			checkoutNewBranchCalled = true
			assert.Equal(t, "main", branch)
			return nil
		},
	}
	gitClientFactory = func() IGitClient { return mockGit }
	defer func() { gitClientFactory = origGitFactory }()

	// Mock resolveWorkspace by changing cwd to tmpDir
	origCwd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origCwd)

	t.Run("Missing Commit Flag", func(t *testing.T) {
		cmd := &cobra.Command{RunE: rollbackCmd.RunE}
		rollbackCommit = ""
		err := cmd.RunE(cmd, []string{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "--commit is required")
	})

	t.Run("Missing Force Flag", func(t *testing.T) {
		cmd := &cobra.Command{RunE: rollbackCmd.RunE}
		rollbackCommit = "abc1234"
		rollbackForce = false
		err := cmd.RunE(cmd, []string{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Use --force to confirm rollback")
	})

	t.Run("Valid Rollback", func(t *testing.T) {
		cmd := &cobra.Command{RunE: rollbackCmd.RunE}
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)

		rollbackCommit = "abc1234"
		rollbackForce = true
		err := cmd.RunE(cmd, []string{})

		assert.NoError(t, err)
		assert.True(t, checkoutCalled)
		assert.True(t, checkoutNewBranchCalled)

		// Verify files are deleted
		_, err = os.Stat(filepath.Join(tmpDir, ".agent_state.json"))
		assert.True(t, os.IsNotExist(err))

		_, err = os.Stat(filepath.Join(tmpDir, ".recac.db"))
		assert.True(t, os.IsNotExist(err))
	})
}
