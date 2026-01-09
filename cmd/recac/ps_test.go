package main

import (
	"os"
	"path/filepath"
	"recac/internal/runner"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestPsAndListCommands(t *testing.T) {
	// This is a dummy usage of the strings package to satisfy the compiler.
	_ = strings.ToUpper("dummy")

	tempDir := t.TempDir()
	sessionDir := filepath.Join(tempDir, "sessions")
	os.MkdirAll(sessionDir, 0755)

	oldSessionManager := sessionManagerFactory
	sessionManagerFactory = func() (ISessionManager, error) {
		return runner.NewSessionManagerWithDir(sessionDir)
	}
	defer func() { sessionManagerFactory = oldSessionManager }()

	t.Run("ps command with no sessions", func(t *testing.T) {
		output, err := executeCommand(rootCmd, "ps")
		require.NoError(t, err)
		require.Contains(t, output, "No sessions found.")
	})

	t.Run("list alias with no sessions", func(t *testing.T) {
		output, err := executeCommand(rootCmd, "list")
		require.NoError(t, err)
		require.Contains(t, output, "No sessions found.")
	})

	// Setup a session with an error
	sm, err := runner.NewSessionManagerWithDir(sessionDir)
	require.NoError(t, err)
	session := &runner.SessionState{
		Name:      "test-session",
		Status:    "error",
		StartTime: time.Now(),
		EndTime:   time.Now(),
		Error:     "something went wrong\nsecond line",
	}
	err = sm.SaveSession(session)
	require.NoError(t, err)

	t.Run("ps command with error session", func(t *testing.T) {
		output, err := executeCommand(rootCmd, "ps")
		require.NoError(t, err)
		require.Contains(t, output, "test-session")
		require.NotContains(t, output, "something went wrong")
	})

	t.Run("ps command with --errors flag", func(t *testing.T) {
		output, err := executeCommand(rootCmd, "ps", "--errors")
		require.NoError(t, err)
		require.Contains(t, output, "test-session")
		require.Contains(t, output, "something went wrong")
		require.NotContains(t, output, "second line")
	})
}
