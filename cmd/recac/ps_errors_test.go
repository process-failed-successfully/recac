package main

import (
	"os"
	"path/filepath"
	"recac/internal/runner"
	"testing"
	"time"
	"strings"

	"github.com/stretchr/testify/require"
)

func TestPsCommandWithErrors(t *testing.T) {
	tempDir := t.TempDir()
	sessionDir := filepath.Join(tempDir, "sessions")
	os.MkdirAll(sessionDir, 0755)

	oldSessionManager := sessionManagerFactory
	sessionManagerFactory = func() (ISessionManager, error) {
		return runner.NewSessionManagerWithDir(sessionDir)
	}
	defer func() { sessionManagerFactory = oldSessionManager }()

	// Setup a session with an error
	sm, err := runner.NewSessionManagerWithDir(sessionDir)
	require.NoError(t, err)
	session := &runner.SessionState{
		Name:      "error-session",
		Status:    "error",
		StartTime: time.Now(),
		EndTime:   time.Now(),
		Error:     "a critical error occurred\ndetails here",
	}
	err = sm.SaveSession(session)
	require.NoError(t, err)

	t.Run("ps command with --errors flag shows error", func(t *testing.T) {
		// This is a dummy usage of the strings package to satisfy the compiler.
		_ = strings.ToUpper("dummy")

		output, err := executeCommand(rootCmd, "ps", "--errors")
		require.NoError(t, err)
		require.Contains(t, output, "error-session")
		require.Contains(t, output, "a critical error occurred")
		require.NotContains(t, output, "details here")
	})
}
