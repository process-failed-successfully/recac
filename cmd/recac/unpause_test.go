package main

import (
	"fmt"
	"recac/internal/runner"
	"testing"

	"github.com/AlecAivazis/survey/v2"
	"github.com/stretchr/testify/require"
)

func TestUnpauseCmd(t *testing.T) {
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

	t.Run("successfully unpauses a paused session", func(t *testing.T) {
		// WHEN the 'unpause' command is executed for a paused session
		output, err := executeCommand(rootCmd, "unpause", "paused-session")

		// THEN there should be no error and the output should confirm the action
		require.NoError(t, err)
		require.Contains(t, output, "Session 'paused-session' unpaused successfully")

		// AND the session status should be updated to "running"
		session, _ := mockSM.LoadSession("paused-session")
		require.Equal(t, "running", session.Status)
	})

	t.Run("fails to unpause a session that is not paused", func(t *testing.T) {
		// WHEN the 'unpause' command is executed for a running session
		_, err := executeCommand(rootCmd, "unpause", "running-session")

		// THEN an error should be returned
		require.Error(t, err)
		require.Contains(t, err.Error(), "session is not paused")
	})

	t.Run("fails to unpause a non-existent session", func(t *testing.T) {
		// WHEN the 'unpause' command is executed for a non-existent session
		_, err := executeCommand(rootCmd, "unpause", "non-existent-session")

		// THEN an error should be returned
		require.Error(t, err)
		require.Contains(t, err.Error(), "session not found")
	})

	t.Run("interactive prompt when no session is provided", func(t *testing.T) {
		// GIVEN the interactive prompt will select "paused-session"
		mockSM.Sessions["paused-session"].Status = "paused" // Ensure it is paused
		oldAskOne := surveyAskOne
		surveyAskOne = func(p survey.Prompt, response interface{}, opts ...survey.AskOpt) error {
			val, ok := response.(*string)
			if !ok {
				return fmt.Errorf("test survey mock expected *string, got %T", response)
			}
			*val = "paused-session"
			return nil
		}
		defer func() { surveyAskOne = oldAskOne }()

		// WHEN the 'unpause' command is executed without arguments
		output, err := executeCommand(rootCmd, "unpause")

		// THEN the command should succeed and unpause the selected session
		require.NoError(t, err)
		require.Contains(t, output, "Session 'paused-session' unpaused successfully")
		session, _ := mockSM.LoadSession("paused-session")
		require.Equal(t, "running", session.Status)
	})

	t.Run("interactive prompt with no paused sessions", func(t *testing.T) {
		// GIVEN all sessions are not in a 'paused' state
		mockSM.Sessions["running-session"].Status = "running"
		mockSM.Sessions["paused-session"].Status = "completed"

		// WHEN the 'unpause' command is executed without arguments
		_, err := executeCommand(rootCmd, "unpause")

		// THEN an error should be returned indicating no sessions are available to unpause
		require.Error(t, err)
		require.EqualError(t, err, fmt.Sprintf("no sessions with status '%s' to select", "paused"))
	})
}
