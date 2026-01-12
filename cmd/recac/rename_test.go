package main

import (
	"testing"
	"recac/internal/runner"

	"github.com/stretchr/testify/require"
)

func TestRenameCmd(t *testing.T) {
	rootCmd, _, _ := newRootCmd()

	// Keep the original factory
	originalFactory := sessionManagerFactory

	// Defer restoration of the original factory
	defer func() {
		sessionManagerFactory = originalFactory
	}()

	t.Run("rename a session", func(t *testing.T) {
		mockSM := NewMockSessionManager()
		mockSM.Sessions["session1"] = &runner.SessionState{Name: "session1", Status: "completed"}
		sessionManagerFactory = func() (ISessionManager, error) {
			return mockSM, nil
		}

		output, err := executeCommand(rootCmd, "rename", "session1", "new-session-name")
		require.NoError(t, err)
		require.Contains(t, output, "Renamed session 'session1' to 'new-session-name'")
		_, exists := mockSM.Sessions["session1"]
		require.False(t, exists, "session1 should be removed")
		_, exists = mockSM.Sessions["new-session-name"]
		require.True(t, exists, "new-session-name should exist")
	})

	t.Run("attempt to rename a running session", func(t *testing.T) {
		mockSM := NewMockSessionManager()
		mockSM.Sessions["session2"] = &runner.SessionState{Name: "session2", Status: "running", PID: 456}
		sessionManagerFactory = func() (ISessionManager, error) {
			return mockSM, nil
		}

		output, err := executeCommand(rootCmd, "rename", "session2", "new-session-name")
		require.Error(t, err)
		require.Empty(t, output) // Expect no output on error
		require.Contains(t, err.Error(), "cannot rename running session 'session2'. Please stop it first")
	})

	t.Run("attempt to rename a non-existent session", func(t *testing.T) {
		mockSM := NewMockSessionManager()
		sessionManagerFactory = func() (ISessionManager, error) {
			return mockSM, nil
		}

		_, err := executeCommand(rootCmd, "rename", "non-existent-session", "new-name")
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to rename session 'non-existent-session'")
	})

	t.Run("not enough arguments", func(t *testing.T) {
		mockSM := NewMockSessionManager()
		sessionManagerFactory = func() (ISessionManager, error) {
			return mockSM, nil
		}
		_, err := executeCommand(rootCmd, "rename", "session1")
		require.Error(t, err)
	})

	t.Run("too many arguments", func(t *testing.T) {
		mockSM := NewMockSessionManager()
		sessionManagerFactory = func() (ISessionManager, error) {
			return mockSM, nil
		}
		_, err := executeCommand(rootCmd, "rename", "session1", "new-name", "extra-arg")
		require.Error(t, err)
	})
}
