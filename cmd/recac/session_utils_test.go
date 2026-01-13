package main

import (
	"os"
	"path/filepath"
	"recac/internal/agent"
	"recac/internal/runner"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestGetFullSessionList(t *testing.T) {
	// --- Setup ---
	// Create a mock session manager
	mockSM := &MockSessionManager{
		Sessions: make(map[string]*runner.SessionState),
	}

	// Create a temporary directory for agent state files
	tempDir := t.TempDir()
	agentStateFileWithCost := filepath.Join(tempDir, "agent_state_with_cost.json")
	err := os.WriteFile(agentStateFileWithCost, []byte(`{
		"model": "claude-3-opus-20240229",
		"token_usage": {
			"total_prompt_tokens": 1000,
			"total_response_tokens": 2000,
			"total_tokens": 3000
		}
	}`), 0644)
	require.NoError(t, err)

	// Create mock sessions
	sessionWithCost := &runner.SessionState{
		Name:           "session-with-cost",
		Status:         "completed",
		StartTime:      time.Now().Add(-1 * time.Hour),
		AgentStateFile: agentStateFileWithCost,
	}
	sessionWithoutCost := &runner.SessionState{
		Name:           "session-no-cost",
		Status:         "running",
		StartTime:      time.Now().Add(-30 * time.Minute),
		AgentStateFile: "non-existent-file.json", // File does not exist
	}
	sessionWithError := &runner.SessionState{
		Name:      "session-error",
		Status:    "error",
		StartTime: time.Now().Add(-2 * time.Hour),
		Error:     "Something went wrong",
	}
	mockSM.Sessions[sessionWithCost.Name] = sessionWithCost
	mockSM.Sessions[sessionWithoutCost.Name] = sessionWithoutCost
	mockSM.Sessions[sessionWithError.Name] = sessionWithError

	t.Run("fetches all local sessions correctly", func(t *testing.T) {
		sessions, err := getFullSessionList(mockSM, false, "")
		require.NoError(t, err)
		require.Len(t, sessions, 3, "should fetch all three mock sessions")

		// Find the session with cost and verify calculation
		var foundWithCost bool
		for _, s := range sessions {
			if s.Name == "session-with-cost" {
				foundWithCost = true
				require.True(t, s.HasCost, "session with agent state should have cost")
				// Cost for claude-3-opus is $15/M prompt, $75/M completion
				// (1000/1,000,000 * 15) + (2000/1,000,000 * 75) = 0.015 + 0.15 = 0.165
				expectedCost := agent.CalculateCost("claude-3-opus-20240229", agent.TokenUsage{TotalPromptTokens: 1000, TotalResponseTokens: 2000, TotalTokens: 3000})
				require.InDelta(t, expectedCost, s.Cost, 0.00001)
				require.Equal(t, 3000, s.Tokens.TotalTokens)
			}
		}
		require.True(t, foundWithCost, "did not find the session that should have cost")
	})

	t.Run("filters sessions by status", func(t *testing.T) {
		sessions, err := getFullSessionList(mockSM, false, "running")
		require.NoError(t, err)
		require.Len(t, sessions, 1, "should only return the running session")
		require.Equal(t, "session-no-cost", sessions[0].Name)
	})

	t.Run("returns empty list for non-matching filter", func(t *testing.T) {
		sessions, err := getFullSessionList(mockSM, false, "non-existent-status")
		require.NoError(t, err)
		require.Len(t, sessions, 0)
	})

	t.Run("handles session manager errors", func(t *testing.T) {
		errorSM := &MockSessionManager{
			Sessions:   make(map[string]*runner.SessionState),
			FailOnList: true,
		}
		_, err := getFullSessionList(errorSM, false, "")
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to list sessions")
	})
}
