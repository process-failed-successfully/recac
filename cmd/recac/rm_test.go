package main

import (
	"fmt"
	"recac/internal/runner"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRmCmd(t *testing.T) {
	// Helper to create a new root command with a mock session manager for each test
	setup := func(t *testing.T) (*MockSessionManager, func(args ...string) (string, error)) {
		mockSM := NewMockSessionManager()
		// Inject the mock factory
		oldFactory := sessionManagerFactory
		sessionManagerFactory = func() (ISessionManager, error) {
			return mockSM, nil
		}
		t.Cleanup(func() {
			sessionManagerFactory = oldFactory
		})

		return mockSM, func(args ...string) (string, error) {
			return executeCommand(rootCmd, args...)
		}
	}

	t.Run("remove a single completed session", func(t *testing.T) {
		mockSM, runCmd := setup(t)
		mockSM.Sessions["session-1"] = &runner.SessionState{Name: "session-1", Status: "completed"}

		output, err := runCmd("rm", "session-1")
		require.NoError(t, err)
		require.Contains(t, output, "Removed session: session-1")

		_, exists := mockSM.Sessions["session-1"]
		require.False(t, exists, "session should have been removed from the mock manager")
	})

	t.Run("remove multiple sessions", func(t *testing.T) {
		mockSM, runCmd := setup(t)
		mockSM.Sessions["session-1"] = &runner.SessionState{Name: "session-1", Status: "completed"}
		mockSM.Sessions["session-2"] = &runner.SessionState{Name: "session-2", Status: "stopped"}

		output, err := runCmd("rm", "session-1", "session-2")
		require.NoError(t, err)
		require.Contains(t, output, "Removed session: session-1")
		require.Contains(t, output, "Removed session: session-2")

		require.Empty(t, mockSM.Sessions, "all sessions should have been removed")
	})

	t.Run("attempt to remove a non-existent session", func(t *testing.T) {
		_, runCmd := setup(t)

		output, err := runCmd("rm", "non-existent")
		require.Error(t, err)
		require.Contains(t, output, "session 'non-existent' not found")
	})

	t.Run("attempt to remove a running session without force", func(t *testing.T) {
		mockSM, runCmd := setup(t)
		mockSM.Sessions["running-session"] = &runner.SessionState{Name: "running-session", Status: "running", PID: 1234}

		output, err := runCmd("rm", "running-session")
		require.Error(t, err)
		require.Contains(t, output, "cannot remove running session 'running-session' without --force")

		_, exists := mockSM.Sessions["running-session"]
		require.True(t, exists, "running session should not be removed without force")
	})

	t.Run("remove a running session with force", func(t *testing.T) {
		mockSM, runCmd := setup(t)
		mockSM.Sessions["running-session"] = &runner.SessionState{Name: "running-session", Status: "running", PID: 1234, StartTime: time.Now()}

		output, err := runCmd("rm", "--force", "running-session")
		require.NoError(t, err)
		require.Contains(t, output, "Removed session: running-session")

		_, exists := mockSM.Sessions["running-session"]
		require.False(t, exists, "session should have been removed")

		// Verify it was marked as "stopped" before removal (part of the mock logic)
		// We can't directly check the state before deletion, but we know StopSession was called.
	})

	t.Run("remove a mix of valid, running, and invalid sessions", func(t *testing.T) {
		mockSM, runCmd := setup(t)
		mockSM.Sessions["completed-1"] = &runner.SessionState{Name: "completed-1", Status: "completed"}
		mockSM.Sessions["running-1"] = &runner.SessionState{Name: "running-1", Status: "running", PID: 123}

		output, err := runCmd("rm", "completed-1", "non-existent", "running-1")

		require.Error(t, err, "command should return an error because some removals failed")

		// Check output for successes and failures
		fmt.Println(output)
		require.Contains(t, output, "Removed session: completed-1")
		require.Contains(t, output, "session 'non-existent' not found")
		require.Contains(t, output, "cannot remove running session 'running-1' without --force")

		// Verify state
		_, existsCompleted := mockSM.Sessions["completed-1"]
		require.False(t, existsCompleted, "completed session should be removed")

		_, existsRunning := mockSM.Sessions["running-1"]
		require.True(t, existsRunning, "running session should NOT be removed")
	})

	t.Run("no arguments provided", func(t *testing.T) {
		_, runCmd := setup(t)
		output, err := runCmd("rm")
		require.Error(t, err)
		// Cobra's default error for missing args
		require.True(t, strings.Contains(output, "requires at least 1 arg(s), only received 0"))
	})
}
