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
	stats, err := calculateStats(sm, time.Time{}, time.Time{}) // No time filter
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

func TestCalculateStats_TimeFilter(t *testing.T) {
	tmpDir := t.TempDir()
	createAgentStateFile := func(name string, model string, tokens int) string {
		state := agent.State{Model: model, TokenUsage: agent.TokenUsage{TotalTokens: tokens}}
		filePath := filepath.Join(tmpDir, name+"_agent_state.json")
		data, _ := json.Marshal(state)
		os.WriteFile(filePath, data, 0644)
		return filePath
	}

	mockSessions := []*runner.SessionState{
		{
			Name:           "recent-session",
			Status:         "completed",
			StartTime:      time.Now().Add(-1 * time.Hour), // 1 hour ago
			AgentStateFile: createAgentStateFile("s1", "gpt-4-turbo", 1000),
		},
		{
			Name:      "boundary-session",
			Status:    "running",
			StartTime: time.Now().Add(-48 * time.Hour), // Exactly 2 days ago
		},
		{
			Name:           "old-session",
			Status:         "completed",
			StartTime:      time.Now().Add(-10 * 24 * time.Hour), // 10 days ago
			AgentStateFile: createAgentStateFile("s3", "gemini-1.5-pro-latest", 5000),
		},
	}

	sessionsMap := make(map[string]*runner.SessionState)
	for _, s := range mockSessions {
		sessionsMap[s.Name] = s
	}
	sm := &MockSessionManager{Sessions: sessionsMap}

	t.Run("since filter", func(t *testing.T) {
		// Filter for sessions since 3 days ago, should include recent and boundary sessions
		sinceTime := time.Now().Add(-3 * 24 * time.Hour)
		stats, err := calculateStats(sm, sinceTime, time.Time{}) // No until time
		require.NoError(t, err)

		require.Equal(t, 2, stats.TotalSessions)
		require.Equal(t, 1000, stats.TotalTokens) // Only from recent-session
		require.Equal(t, 1, stats.StatusCounts["completed"])
		require.Equal(t, 1, stats.StatusCounts["running"])
	})

	t.Run("until filter", func(t *testing.T) {
		// Filter for sessions until 3 days ago, should only include old-session
		untilTime := time.Now().Add(-3 * 24 * time.Hour)
		stats, err := calculateStats(sm, time.Time{}, untilTime) // No since time
		require.NoError(t, err)

		require.Equal(t, 1, stats.TotalSessions)
		require.Equal(t, 5000, stats.TotalTokens) // Only from old-session
		require.Equal(t, 1, stats.StatusCounts["completed"])
		require.NotContains(t, stats.StatusCounts, "running")
	})

	t.Run("since and until filter", func(t *testing.T) {
		// Filter for a window between 5 days and 1 day ago, should only include boundary-session
		sinceTime := time.Now().Add(-5 * 24 * time.Hour)
		untilTime := time.Now().Add(-1 * 24 * time.Hour)
		stats, err := calculateStats(sm, sinceTime, untilTime)
		require.NoError(t, err)

		require.Equal(t, 1, stats.TotalSessions)
		require.Equal(t, 0, stats.TotalTokens) // boundary-session has no agent state
		require.Equal(t, 1, stats.StatusCounts["running"])
		require.NotContains(t, stats.StatusCounts, "completed")
	})

	t.Run("no results", func(t *testing.T) {
		// Filter for a window that contains no sessions
		sinceTime := time.Now().Add(-20 * 24 * time.Hour)
		untilTime := time.Now().Add(-15 * 24 * time.Hour)
		stats, err := calculateStats(sm, sinceTime, untilTime)
		require.NoError(t, err)

		require.Equal(t, 0, stats.TotalSessions)
		require.Equal(t, 0, stats.TotalTokens)
	})
}
