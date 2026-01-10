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

// Helper to create agent state files for testing
func createAgentStateFile(t *testing.T, dir string, name string, model string, promptTokens, responseTokens int) string {
	state := agent.State{
		Model: model,
		TokenUsage: agent.TokenUsage{
			TotalPromptTokens:   promptTokens,
			TotalResponseTokens: responseTokens,
			TotalTokens:         promptTokens + responseTokens,
		},
	}
	filePath := filepath.Join(dir, name+"_agent_state.json")
	data, err := json.Marshal(state)
	require.NoError(t, err)
	os.WriteFile(filePath, data, 0644)
	return filePath
}

func setupMockSessions(t *testing.T, tmpDir string) *MockSessionManager {
	mockSessions := []*runner.SessionState{
		{
			Name:           "session1-completed-gemini",
			Status:         "COMPLETED",
			AgentStateFile: createAgentStateFile(t, tmpDir, "s1", "gemini-1.5-pro-latest", 100, 200),
			StartTime:      time.Now().Add(-1 * time.Hour),
		},
		{
			Name:           "session2-completed-claude",
			Status:         "COMPLETED",
			AgentStateFile: createAgentStateFile(t, tmpDir, "s2", "claude-3-opus-20240229", 50, 150),
			StartTime:      time.Now().Add(-25 * time.Hour),
		},
		{
			Name:      "session3-running",
			Status:    "RUNNING",
			PID:       123, // Mock PID
			StartTime: time.Now().Add(-10 * time.Minute),
		},
		{
			Name:           "session4-failed",
			Status:         "FAILED",
			AgentStateFile: createAgentStateFile(t, tmpDir, "s4", "gemini-1.5-pro-latest", 10, 20),
			StartTime:      time.Now().Add(-3 * 24 * time.Hour),
		},
		{
			Name:           "session5-no-state",
			Status:         "COMPLETED",
			AgentStateFile: "", // No agent state
			StartTime:      time.Now().Add(-2 * time.Hour),
		},
	}

	sessionsMap := make(map[string]*runner.SessionState)
	for _, s := range mockSessions {
		sessionsMap[s.Name] = s
	}

	return &MockSessionManager{
		Sessions: sessionsMap,
	}
}

func TestCalculateStats_NoFilters(t *testing.T) {
	tmpDir := t.TempDir()
	sm := setupMockSessions(t, tmpDir)

	stats, err := calculateStats(sm, "", "", "")
	require.NoError(t, err)

	require.Equal(t, 5, stats.TotalSessions)
	require.Equal(t, 3, stats.StatusCounts["COMPLETED"])
	require.Equal(t, 1, stats.StatusCounts["RUNNING"])
	require.Equal(t, 1, stats.StatusCounts["FAILED"])
	require.Equal(t, 530, stats.TotalTokens) // 300 + 200 + 30
	require.InDelta(t, 0.01739, stats.TotalCost, 0.00001)
}

func TestCalculateStatsWithFilters(t *testing.T) {
	tmpDir := t.TempDir()
	sm := setupMockSessions(t, tmpDir)

	t.Run("Filter by status=COMPLETED", func(t *testing.T) {
		stats, err := calculateStats(sm, "COMPLETED", "", "")
		require.NoError(t, err)

		require.Equal(t, 3, stats.TotalSessions)
		require.Equal(t, 3, stats.StatusCounts["COMPLETED"])
		require.Equal(t, 0, stats.StatusCounts["RUNNING"])
		require.Equal(t, 500, stats.TotalTokens) // s1 + s2
	})

	t.Run("Filter by since=3h", func(t *testing.T) {
		stats, err := calculateStats(sm, "", "3h", "")
		require.NoError(t, err)

		require.Equal(t, 3, stats.TotalSessions) // s1, s3, s5
		require.Equal(t, 2, stats.StatusCounts["COMPLETED"])
		require.Equal(t, 1, stats.StatusCounts["RUNNING"])
		require.Equal(t, 300, stats.TotalTokens) // s1 only
	})

	t.Run("Filter by model=gemini-1.5-pro-latest", func(t *testing.T) {
		stats, err := calculateStats(sm, "", "", "gemini-1.5-pro-latest")
		require.NoError(t, err)

		require.Equal(t, 2, stats.TotalSessions) // s1, s4
		require.Equal(t, 1, stats.StatusCounts["COMPLETED"])
		require.Equal(t, 1, stats.StatusCounts["FAILED"])
		require.Equal(t, 330, stats.TotalTokens) // 300 + 30
	})

	t.Run("Filter by status and since", func(t *testing.T) {
		stats, err := calculateStats(sm, "COMPLETED", "3h", "")
		require.NoError(t, err)

		require.Equal(t, 2, stats.TotalSessions) // s1, s5
		require.Equal(t, 2, stats.StatusCounts["COMPLETED"])
		require.Equal(t, 300, stats.TotalTokens) // s1 only
	})

	t.Run("Filter by model and since", func(t *testing.T) {
		stats, err := calculateStats(sm, "", "30h", "gemini-1.5-pro-latest")
		require.NoError(t, err)

		require.Equal(t, 1, stats.TotalSessions) // s1 only
		require.Equal(t, 1, stats.StatusCounts["COMPLETED"])
		require.Equal(t, 300, stats.TotalTokens)
	})

	t.Run("No results", func(t *testing.T) {
		stats, err := calculateStats(sm, "NON_EXISTENT_STATUS", "", "")
		require.NoError(t, err)
		require.Equal(t, 0, stats.TotalSessions)
		require.Equal(t, 0.0, stats.TotalCost)
	})
}
