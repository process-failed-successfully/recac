package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"recac/internal/runner"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupLogsTest creates a temporary directory with mock session files and log files.
func setupLogsTest(t *testing.T) (string, func()) {
	t.Helper()

	tempDir, err := os.MkdirTemp("", "logs-test-*")
	require.NoError(t, err)

	sessionsDir := filepath.Join(tempDir, ".recac", "sessions")
	require.NoError(t, os.MkdirAll(sessionsDir, 0755))

	// Session 1: Older, completed
	session1LogPath := filepath.Join(sessionsDir, "session-1-completed.log")
	session1 := &runner.SessionState{
		Name:      "session-1-completed",
		Status:    "completed",
		StartTime: time.Now().Add(-2 * time.Hour),
		LogFile:   session1LogPath,
	}
	session1Data, _ := json.Marshal(session1)
	require.NoError(t, os.WriteFile(filepath.Join(sessionsDir, "session-1-completed.json"), session1Data, 0644))
	require.NoError(t, os.WriteFile(session1LogPath, []byte("Log from completed session 1\n"), 0644))

	// Session 2: Running
	session2LogPath := filepath.Join(sessionsDir, "session-2-running.log")
	session2 := &runner.SessionState{
		Name:      "session-2-running",
		Status:    "running",
		StartTime: time.Now().Add(-1 * time.Hour),
		LogFile:   session2LogPath,
		PID:       os.Getpid(), // Use a real PID to prevent status change
	}
	session2Data, _ := json.Marshal(session2)
	require.NoError(t, os.WriteFile(filepath.Join(sessionsDir, "session-2-running.json"), session2Data, 0644))
	require.NoError(t, os.WriteFile(session2LogPath, []byte("Log from running session 2\n"), 0644))

	// Session 3: Newer, completed (should be the default)
	session3LogPath := filepath.Join(sessionsDir, "session-3-last-completed.log")
	session3 := &runner.SessionState{
		Name:      "session-3-last-completed",
		Status:    "completed",
		StartTime: time.Now().Add(-30 * time.Minute),
		LogFile:   session3LogPath,
	}
	session3Data, _ := json.Marshal(session3)
	require.NoError(t, os.WriteFile(filepath.Join(sessionsDir, "session-3-last-completed.json"), session3Data, 0644))
	require.NoError(t, os.WriteFile(session3LogPath, []byte("Log from last completed session 3\n"), 0644))

	cleanup := func() {
		os.RemoveAll(tempDir)
	}
	return tempDir, cleanup
}
func TestLogsCmd(t *testing.T) {
	tempDir, cleanup := setupLogsTest(t)
	defer cleanup()

	t.Setenv("HOME", tempDir)

	origExit := exit
	defer func() { exit = origExit }()
	exit = func(code int) {
		if code != 0 {
			panic(code) // Panic with the exit code
		}
	}

	t.Run("LogsForSpecificSession", func(t *testing.T) {
		stdout, stderr := captureOutput(t, func() {
			rootCmd.SetArgs([]string{"logs", "session-1-completed"})
			err := rootCmd.Execute()
			assert.NoError(t, err)
		})
		assert.Contains(t, stdout, "Log from completed session 1")
		assert.NotContains(t, stdout, "showing logs for last completed session")
		assert.NotContains(t, stderr, "Error:")
	})

	t.Run("LogsForLastCompletedSession", func(t *testing.T) {
		stdout, stderr := captureOutput(t, func() {
			rootCmd.SetArgs([]string{"logs"})
			err := rootCmd.Execute()
			assert.NoError(t, err)
		})
		assert.Contains(t, stdout, "No session name provided, showing logs for last completed session: session-3-last-completed")
		assert.Contains(t, stdout, "Log from last completed session 3")
		assert.NotContains(t, stderr, "Error:")
	})

	t.Run("LogsForNonExistentSession", func(t *testing.T) {
		oldStderr := os.Stderr
		r, w, _ := os.Pipe()
		os.Stderr = w

		assert.PanicsWithValue(t, 1, func() {
			rootCmd.SetArgs([]string{"logs", "non-existent-session"})
			_ = rootCmd.Execute()
		}, "Command should have panicked with exit code 1")

		w.Close()
		os.Stderr = oldStderr
		var stderrBuf bytes.Buffer
		io.Copy(&stderrBuf, r)

		assert.Contains(t, stderrBuf.String(), "Error: session not found")
	})

	t.Run("NoCompletedSessionsFound", func(t *testing.T) {
		// Create a temporary directory with only a running session
		emptyDir, emptyCleanup := setupLogsTest(t)
		defer emptyCleanup()
		require.NoError(t, os.Remove(filepath.Join(emptyDir, ".recac", "sessions", "session-1-completed.json")))
		require.NoError(t, os.Remove(filepath.Join(emptyDir, ".recac", "sessions", "session-3-last-completed.json")))
		t.Setenv("HOME", emptyDir)

		oldStderr := os.Stderr
		r, w, _ := os.Pipe()
		os.Stderr = w

		assert.PanicsWithValue(t, 1, func() {
			rootCmd.SetArgs([]string{"logs"})
			_ = rootCmd.Execute()
		}, "Command should have panicked with exit code 1")

		w.Close()
		os.Stderr = oldStderr
		var stderrBuf bytes.Buffer
		io.Copy(&stderrBuf, r)

		assert.Contains(t, stderrBuf.String(), "Error: No completed sessions found.")
	})
}
