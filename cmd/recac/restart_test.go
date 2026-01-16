package main

import (
	"fmt"
	"recac/internal/runner"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRestartCmd(t *testing.T) {
	// successful restart
	t.Run("success", func(t *testing.T) {
		mockSM := NewMockSessionManager()
		sessionName := "test-session"
		originalSession := &runner.SessionState{
			Name:      sessionName,
			PID:       1234,
			Status:    "completed",
			Command:   []string{"/bin/sleep", "10"},
			Workspace: "/tmp",
		}
		mockSM.Sessions[sessionName] = originalSession

		sessionManagerFactory = func() (ISessionManager, error) {
			return mockSM, nil
		}

		output, err := executeCommand(rootCmd, "restart", sessionName)

		assert.NoError(t, err)
		assert.Contains(t, output, fmt.Sprintf("Session '%s' restarted successfully (PID: %d)\n", sessionName, 99999))

		// Verify the session was "restarted" in the mock
		restartedSession, exists := mockSM.Sessions[sessionName]
		assert.True(t, exists, "Session should still exist in mock manager")
		assert.Equal(t, "running", restartedSession.Status)
		assert.Equal(t, 99999, restartedSession.PID) // Mock PID from helper
	})

	// error when session is running
	t.Run("error on running session", func(t *testing.T) {
		mockSM := NewMockSessionManager()
		sessionName := "running-session"
		runningSession := &runner.SessionState{
			Name:   sessionName,
			PID:    1111,
			Status: "running",
		}
		mockSM.Sessions[sessionName] = runningSession
		// This is the critical part: ensure the mock knows the PID is for a running process.
		mockSM.IsProcessRunningFunc = func(pid int) bool {
			return pid == 1111
		}

		sessionManagerFactory = func() (ISessionManager, error) {
			return mockSM, nil
		}

		_, err := executeCommand(rootCmd, "restart", sessionName)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), fmt.Sprintf("cannot restart a running session: '%s' (PID: %d)", runningSession.Name, runningSession.PID))
	})

	// error on non-existent session
	t.Run("error on non-existent session", func(t *testing.T) {
		mockSM := NewMockSessionManager()
		sessionName := "non-existent-session"

		sessionManagerFactory = func() (ISessionManager, error) {
			return mockSM, nil
		}

		_, err := executeCommand(rootCmd, "restart", sessionName)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to load session")
	})
}
