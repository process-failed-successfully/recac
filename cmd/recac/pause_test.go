package main

import (
	"recac/internal/runner"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPauseCmd(t *testing.T) {
	// GIVEN a root command, a mock session manager, and a factory override
	rootCmd, _, _ := newRootCmd()
	mockSM := NewMockSessionManager()
	oldFactory := sessionManagerFactory
	sessionManagerFactory = func() (ISessionManager, error) {
		return mockSM, nil
	}
	defer func() { sessionManagerFactory = oldFactory }()

	// AND the following sessions
	mockSM.Sessions["running-session"] = &runner.SessionState{Name: "running-session", Status: "running", PID: 123}
	mockSM.Sessions["paused-session"] = &runner.SessionState{Name: "paused-session", Status: "paused", PID: 456}

	t.Run("successfully pauses a running session", func(t *testing.T) {
		// WHEN the 'pause' command is executed for a running session
		output, err := executeCommand(rootCmd, "pause", "running-session")

		// THEN there should be no error and the output should confirm the pause
		require.NoError(t, err)
		require.Contains(t, output, "Session 'running-session' paused successfully")

		// AND the session status should be updated to "paused"
		session, _ := mockSM.LoadSession("running-session")
		require.Equal(t, "paused", session.Status)
	})

	t.Run("fails to pause a session that is not running", func(t *testing.T) {
		// WHEN the 'pause' command is executed for an already paused session
		_, err := executeCommand(rootCmd, "pause", "paused-session")

		// THEN an error should be returned
		require.Error(t, err)
		require.Contains(t, err.Error(), "session is not running")
	})

	t.Run("fails to pause a non-existent session", func(t *testing.T) {
		// WHEN the 'pause' command is executed for a non-existent session
		_, err := executeCommand(rootCmd, "pause", "non-existent-session")

		// THEN an error should be returned
		require.Error(t, err)
		require.Contains(t, err.Error(), "session not found")
	})

	t.Run("fails when no session is provided", func(t *testing.T) {
		// WHEN the 'pause' command is executed without arguments
		_, err := executeCommand(rootCmd, "pause")

		// THEN an error should be returned
		require.Error(t, err)
		require.Contains(t, err.Error(), "session name is required")
	})
}
