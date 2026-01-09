package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"recac/internal/agent"
	"recac/internal/runner"

	"github.com/stretchr/testify/require"
)

func TestCalculateStats(t *testing.T) {
	tmpDir := t.TempDir()

	// Helper to create agent state files
	createAgentStateFile := func(name string, model string, promptTokens, responseTokens int) string {
		state := agent.State{
			Model: model,
			TokenUsage: agent.TokenUsage{
				TotalPromptTokens:   promptTokens,
				TotalResponseTokens: responseTokens,
				TotalTokens:         promptTokens + responseTokens,
			},
		}
		filePath := filepath.Join(tmpDir, name+"_agent_state.json")
		data, err := json.Marshal(state)
		require.NoError(t, err)
		os.WriteFile(filePath, data, 0644)
		return filePath
	}

	mockSessions := []*runner.SessionState{
		{
			Name:           "session1-completed",
			Status:         "completed",
			AgentStateFile: createAgentStateFile("s1", "gemini-1.5-pro-latest", 100, 200),
			StartTime:      time.Now(),
		},
		{
			Name:           "session2-completed",
			Status:         "completed",
			AgentStateFile: createAgentStateFile("s2", "claude-3-opus-20240229", 50, 150),
			StartTime:      time.Now(),
		},
		{
			Name:      "session3-running",
			Status:    "running",
			PID:       123, // Mock PID
			StartTime: time.Now(),
		},
		{
			Name:           "session4-failed-no-state",
			Status:         "failed",
			AgentStateFile: "", // No agent state
			StartTime:      time.Now(),
		},
	}

	// Convert slice to map for the mock
	sessionsMap := make(map[string]*runner.SessionState)
	for _, s := range mockSessions {
		sessionsMap[s.Name] = s
	}

	sm := &MockSessionManager{
		Sessions: sessionsMap,
	}

	// Calculate stats
	stats, err := calculateStats(sm)
	require.NoError(t, err)

	// --- Assertions ---
	require.Equal(t, 4, stats.TotalSessions, "Total sessions should be 4")
	require.Equal(t, 500, stats.TotalTokens, "Total tokens should be sum of s1 and s2")
	require.Equal(t, 150, stats.TotalPromptTokens, "Total prompt tokens should be sum of s1 and s2")
	require.Equal(t, 350, stats.TotalResponseTokens, "Total response tokens should be sum of s1 and s2")

	cost1 := agent.CalculateCost("gemini-1.5-pro-latest", agent.TokenUsage{TotalPromptTokens: 100, TotalResponseTokens: 200})
	cost2 := agent.CalculateCost("claude-3-opus-20240229", agent.TokenUsage{TotalPromptTokens: 50, TotalResponseTokens: 150})
	expectedCost := cost1 + cost2
	require.InDelta(t, expectedCost, stats.TotalCost, 0.0001, "Total cost should be sum of s1 and s2")

	require.Equal(t, 2, stats.StatusCounts["completed"], "Should have 2 completed sessions")
	require.Equal(t, 1, stats.StatusCounts["running"], "Should have 1 running session")
	require.Equal(t, 1, stats.StatusCounts["failed"], "Should have 1 failed session")
}
