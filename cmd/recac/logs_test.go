package main

import (
	"os"
	"path/filepath"
	"recac/internal/runner"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupLogsTest configures a mock session manager with temporary log files.
func setupLogsTest(t *testing.T) (*MockSessionManager, func()) {
	t.Helper()

	mockSM := NewMockSessionManager()

	// Create a temp dir for log files
	tmpDir, err := os.MkdirTemp("", "recac-logs-test-")
	require.NoError(t, err)

	// --- Create mock sessions and their log files ---
	session1Log := filepath.Join(tmpDir, "session1.log")
	err = os.WriteFile(session1Log, []byte("session 1 log line 1\nsession 1 log line 2\n"), 0644)
	require.NoError(t, err)

	session2Log := filepath.Join(tmpDir, "session2.log")
	err = os.WriteFile(session2Log, []byte("session 2 log line 1\n"), 0644)
	require.NoError(t, err)

	stoppedSessionLog := filepath.Join(tmpDir, "stopped.log")
	err = os.WriteFile(stoppedSessionLog, []byte("this should not be read\n"), 0644)
	require.NoError(t, err)

	mockSM.Sessions = map[string]*runner.SessionState{
		"session1": {
			Name:    "session1",
			Status:  "running",
			LogFile: session1Log,
		},
		"session2": {
			Name:    "session2",
			Status:  "running",
			LogFile: session2Log,
		},
		"stopped-session": {
			Name:    "stopped-session",
			Status:  "stopped",
			LogFile: stoppedSessionLog,
		},
	}

	// Monkey-patch the sessionManagerFactory
	originalFactory := sessionManagerFactory
	sessionManagerFactory = func() (ISessionManager, error) {
		return mockSM, nil
	}

	cleanup := func() {
		os.RemoveAll(tmpDir)
		sessionManagerFactory = originalFactory
	}

	return mockSM, cleanup
}

func TestLogsCmd(t *testing.T) {
	t.Run("logs --all streams running sessions", func(t *testing.T) {
		_, cleanup := setupLogsTest(t)
		defer cleanup()

		output, err := executeCommand(rootCmd, "logs", "--all")
		require.NoError(t, err)

		// Check that output from both running sessions is present
		assert.Contains(t, output, "[session1] session 1 log line 1")
		assert.Contains(t, output, "[session1] session 1 log line 2")
		assert.Contains(t, output, "[session2] session 2 log line 1")

		// Check that output from the stopped session is not present
		assert.NotContains(t, output, "stopped-session")
		assert.NotContains(t, output, "this should not be read")
	})

	t.Run("logs single session", func(t *testing.T) {
		_, cleanup := setupLogsTest(t)
		defer cleanup()

		output, err := executeCommand(rootCmd, "logs", "session1")
		require.NoError(t, err)

		assert.Contains(t, output, "session 1 log line 1")
		assert.Contains(t, output, "session 1 log line 2")
		assert.NotContains(t, output, "[session1]") // No prefix for single log
		assert.NotContains(t, output, "session 2")
	})

	t.Run("logs --all with filter", func(t *testing.T) {
		_, cleanup := setupLogsTest(t)
		defer cleanup()

		output, err := executeCommand(rootCmd, "logs", "--all", "--filter", "line 2")
		require.NoError(t, err)

		assert.Contains(t, output, "[session1] session 1 log line 2")
		assert.NotContains(t, output, "session 1 log line 1")
		assert.NotContains(t, output, "session 2")
	})

	t.Run("logs --all with no running sessions", func(t *testing.T) {
		mockSM, cleanup := setupLogsTest(t)
		defer cleanup()

		// Override to have no running sessions
		mockSM.Sessions["session1"].Status = "completed"
		mockSM.Sessions["session2"].Status = "error"

		output, err := executeCommand(rootCmd, "logs", "--all")
		require.NoError(t, err)

		assert.Contains(t, output, "No running sessions found.")
	})

	t.Run("logs validation errors", func(t *testing.T) {
		_, cleanup := setupLogsTest(t)
		defer cleanup()

		_, err := executeCommand(rootCmd, "logs")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "requires a session name or --all flag")

		_, err = executeCommand(rootCmd, "logs", "session1", "--all")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot use session name with --all")

		_, err = executeCommand(rootCmd, "logs", "non-existent-session")
		// This will print an error and exit(1), which is caught by executeCommand
		// and does not return a Go error. We check stderr.
		output, _ := executeCommand(rootCmd, "logs", "non-existent-session")
		assert.Contains(t, output, "Error: session not found")
	})

	// Note: Testing --follow is complex in unit tests as it involves long-running processes.
	// This is better suited for E2E tests. The core logic of reading and streaming is
	// already tested by the cases above.
}
