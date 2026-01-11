package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"recac/internal/runner"

	"github.com/stretchr/testify/require"
)

func TestSummaryCommand(t *testing.T) {
	t.Run("No sessions", func(t *testing.T) {
		rootCmd, _, _ := newRootCmd()
		mockSm := NewMockSessionManager()
		sessionManagerFactory = func() (ISessionManager, error) {
			return mockSm, nil
		}

		output, err := executeCommand(rootCmd, "summary")
		require.NoError(t, err)

		require.Contains(t, output, "RECAC Status")
		require.Contains(t, output, "No session activity found.")
		require.NotContains(t, output, "Aggregate Stats")
	})

	t.Run("With sessions", func(t *testing.T) {
		tempDir := t.TempDir()
		agentState1 := `{"model": "test-model-1", "token_usage": {"total_prompt_tokens": 100, "total_response_tokens": 150, "total_tokens": 250}}`
		agentStateFile1 := filepath.Join(tempDir, "agent_state_1.json")
		err := os.WriteFile(agentStateFile1, []byte(agentState1), 0600)
		require.NoError(t, err)

		agentState2 := `{"model": "test-model-2", "token_usage": {"total_prompt_tokens": 200, "total_response_tokens": 250, "total_tokens": 450}}`
		agentStateFile2 := filepath.Join(tempDir, "agent_state_2.json")
		err = os.WriteFile(agentStateFile2, []byte(agentState2), 0600)
		require.NoError(t, err)

		rootCmd, _, _ := newRootCmd()
		mockSm := NewMockSessionManager()
		sessionManagerFactory = func() (ISessionManager, error) {
			return mockSm, nil
		}
		mockSm.Sessions = map[string]*runner.SessionState{
			"session-1": {
				Name:           "session-1",
				Status:         "completed",
				StartTime:      time.Now().Add(-1 * time.Hour),
				EndTime:        time.Now().Add(-30 * time.Minute),
				AgentStateFile: agentStateFile1,
			},
			"session-2": {
				Name:           "session-2",
				Status:         "running",
				StartTime:      time.Now().Add(-10 * time.Minute),
				AgentStateFile: agentStateFile2,
			},
			"session-3-no-cost": {
				Name:      "session-3-no-cost",
				Status:    "error",
				StartTime: time.Now().Add(-2 * time.Hour),
				EndTime:   time.Now().Add(-1 * time.Hour),
			},
		}

		// Execute the summary command with a limit of 2 for the lists.
		output, err := executeCommand(rootCmd, "summary", "--limit", "2")
		require.NoError(t, err)

		// Check for the presence of each section and its key content.
		// This is more robust than splitting the string and checking indices.
		require.Contains(t, output, "RECAC Status")

		require.Contains(t, output, "Aggregate Stats")
		require.Regexp(t, `Total Sessions:\s+3`, output)
		require.Regexp(t, `Total Tokens:\s+700`, output) // 250 + 450

		require.Contains(t, output, "Recent Activity")
		require.Contains(t, output, "session-2")
		require.Contains(t, output, "session-1")
		require.NotContains(t, output, "session-3-no-cost") // Should be excluded due to the limit

		require.Contains(t, output, "Top Sessions by Cost")
		require.Contains(t, output, "session-2") // Assumes model-2 is more expensive
		require.Contains(t, output, "session-1")
		require.NotContains(t, output, "session-3-no-cost")
	})
}
