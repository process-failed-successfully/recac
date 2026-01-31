package main

import (
	"os"
	"recac/internal/runner"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// setupShowAliasTest is a helper to create a mock session for testing.
// It reuses the logic from workdiff_test.go but is standalone to avoid cross-file dependency issues during partial test runs.
func setupShowAliasTest(t *testing.T) (sm *runner.SessionManager, sessionName, repoDir string) {
	t.Helper()

	// 1. Setup a temporary git repository (simulated via empty dir as show alias might not need full git for basic validation)
	repoDir, err := os.MkdirTemp("", "recac-show-test-*")
	require.NoError(t, err)

	// 2. Create a mock session
	sessionsDir, err := os.MkdirTemp("", "recac-sessions-*")
	require.NoError(t, err)
	t.Cleanup(func() { os.RemoveAll(sessionsDir) })

	sm, err = runner.NewSessionManagerWithDir(sessionsDir)
	require.NoError(t, err)

	sessionName = "show-test-session"
	session := &runner.SessionState{
		Name:           sessionName,
		Status:         "completed",
		StartTime:      time.Now(),
		EndTime:        time.Now(),
		Workspace:      repoDir,
		StartCommitSHA: "abc",
		EndCommitSHA:   "def",
	}
	err = sm.SaveSession(session)
	require.NoError(t, err)

	return sm, sessionName, repoDir
}

func TestShowAliasCmd_Standalone(t *testing.T) {
	sm, sessionName, repoDir := setupShowAliasTest(t)
	defer os.RemoveAll(repoDir)

	// Mock the session manager factory
	originalFactory := sessionManagerFactory
	sessionManagerFactory = func() (ISessionManager, error) {
		return sm, nil
	}
	defer func() { sessionManagerFactory = originalFactory }()

	// We can't easily mock `newRootCmd` here without importing everything,
    // so we will rely on the fact that `executeCommand` and `rootCmd` are available in the package.
    // However, if we are running this file in isolation, we might miss them.
    // This file is intended to be run as part of the package.

	t.Run("show with one argument should succeed", func(t *testing.T) {
        // We mock the GitClientFactory to return a mock git client that handles Diff
        originalGitFactory := gitClientFactory
        gitClientFactory = func() IGitClient {
            return &MockGitClient{
                DiffFunc: func(repoPath, commitA, commitB string) (string, error) {
                    return "diff --git a/test.txt b/test.txt\n-hello\n+hello world", nil
                },
                RepoExistsFunc: func(repoPath string) bool { return true },
            }
        }
        defer func() { gitClientFactory = originalGitFactory }()

		rootCmd, _, _ := newRootCmd() // Assumes root.go is compiled in
		output, err := executeCommand(rootCmd, "show", sessionName)
		require.NoError(t, err)
		require.Contains(t, output, "diff --git a/test.txt b/test.txt")
	})

	t.Run("show with two arguments should fail", func(t *testing.T) {
		rootCmd, _, _ := newRootCmd()
		_, err := executeCommand(rootCmd, "show", sessionName, "another-session")
		require.Error(t, err)
		require.Contains(t, err.Error(), "requires exactly one session name")
	})

	t.Run("show with no arguments should fail", func(t *testing.T) {
		rootCmd, _, _ := newRootCmd()
		_, err := executeCommand(rootCmd, "show")
		require.Error(t, err)
		require.Contains(t, err.Error(), "requires exactly one session name")
	})
}
