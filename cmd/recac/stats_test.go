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

	// --- Mock Data ---
	mockSessions := []*runner.SessionState{
		{
			Name:           "s1-completed-gemini",
			Status:         "completed",
			AgentStateFile: createAgentStateFile("s1", "gemini-pro", 100, 200),
			StartTime:      time.Now().Add(-2 * time.Hour),
		},
		{
			Name:           "s2-completed-claude",
			Status:         "completed",
			AgentStateFile: createAgentStateFile("s2", "claude-opus", 50, 150),
			StartTime:      time.Now().Add(-12 * time.Hour),
		},
		{
			Name:      "s3-running",
			Status:    "running",
			PID:       123,
			StartTime: time.Now().Add(-10 * time.Minute),
		},
		{
			Name:           "s4-failed-gemini-old",
			Status:         "failed",
			AgentStateFile: createAgentStateFile("s4", "gemini-pro", 10, 20),
			StartTime:      time.Now().Add(-48 * time.Hour),
		},
		{
			Name:           "s5-completed-no-state",
			Status:         "completed",
			AgentStateFile: "",
			StartTime:      time.Now().Add(-1 * time.Hour),
		},
	}
	sessionsMap := make(map[string]*runner.SessionState)
	for _, s := range mockSessions {
		sessionsMap[s.Name] = s
	}
	sm := &MockSessionManager{Sessions: sessionsMap}

	// --- Test Cases ---
	testCases := []struct {
		name                string
		statusFilter        string
		sinceFilter         time.Time
		modelFilter         string
		expectedTotal       int
		expectedTokens      int
		expectedCost        float64
		expectedStatusCount map[string]int
	}{
		{
			name:                "No filters",
			statusFilter:        "",
			sinceFilter:         time.Time{},
			modelFilter:         "",
			expectedTotal:       5,
			expectedTokens:      530, // 300 (s1) + 200 (s2) + 30 (s4)
			expectedCost:        agent.CalculateCost("gemini-pro", agent.TokenUsage{TotalPromptTokens: 100, TotalResponseTokens: 200}) + agent.CalculateCost("claude-opus", agent.TokenUsage{TotalPromptTokens: 50, TotalResponseTokens: 150}) + agent.CalculateCost("gemini-pro", agent.TokenUsage{TotalPromptTokens: 10, TotalResponseTokens: 20}),
			expectedStatusCount: map[string]int{"completed": 3, "running": 1, "failed": 1},
		},
		{
			name:                "Filter by status: completed",
			statusFilter:        "completed",
			sinceFilter:         time.Time{},
			modelFilter:         "",
			expectedTotal:       3,
			expectedTokens:      500, // 300 (s1) + 200 (s2)
			expectedCost:        agent.CalculateCost("gemini-pro", agent.TokenUsage{TotalPromptTokens: 100, TotalResponseTokens: 200}) + agent.CalculateCost("claude-opus", agent.TokenUsage{TotalPromptTokens: 50, TotalResponseTokens: 150}),
			expectedStatusCount: map[string]int{"completed": 3},
		},
		{
			name:                "Filter by since: 24h",
			statusFilter:        "",
			sinceFilter:         time.Now().Add(-24 * time.Hour),
			modelFilter:         "",
			expectedTotal:       4, // s1, s2, s3, s5
			expectedTokens:      500, // 300 (s1) + 200 (s2)
			expectedCost:        agent.CalculateCost("gemini-pro", agent.TokenUsage{TotalPromptTokens: 100, TotalResponseTokens: 200}) + agent.CalculateCost("claude-opus", agent.TokenUsage{TotalPromptTokens: 50, TotalResponseTokens: 150}),
			expectedStatusCount: map[string]int{"completed": 3, "running": 1},
		},
		{
			name:                "Filter by model: gemini-pro",
			statusFilter:        "",
			sinceFilter:         time.Time{},
			modelFilter:         "gemini-pro",
			expectedTotal:       2, // s1, s4
			expectedTokens:      330, // 300 (s1) + 30 (s4)
			expectedCost:        agent.CalculateCost("gemini-pro", agent.TokenUsage{TotalPromptTokens: 100, TotalResponseTokens: 200}) + agent.CalculateCost("gemini-pro", agent.TokenUsage{TotalPromptTokens: 10, TotalResponseTokens: 20}),
			expectedStatusCount: map[string]int{"completed": 1, "failed": 1},
		},
		{
			name:                "Filter by status and since",
			statusFilter:        "completed",
			sinceFilter:         time.Now().Add(-3 * time.Hour),
			modelFilter:         "",
			expectedTotal:       2, // s1, s5
			expectedTokens:      300, // only s1 has tokens
			expectedCost:        agent.CalculateCost("gemini-pro", agent.TokenUsage{TotalPromptTokens: 100, TotalResponseTokens: 200}),
			expectedStatusCount: map[string]int{"completed": 2},
		},
		{
			name:                "Filter by model and since",
			statusFilter:        "",
			sinceFilter:         time.Now().Add(-24 * time.Hour),
			modelFilter:         "gemini-pro",
			expectedTotal:       1, // s1
			expectedTokens:      300,
			expectedCost:        agent.CalculateCost("gemini-pro", agent.TokenUsage{TotalPromptTokens: 100, TotalResponseTokens: 200}),
			expectedStatusCount: map[string]int{"completed": 1},
		},
		{
			name:                "No matching sessions",
			statusFilter:        "running",
			sinceFilter:         time.Now().Add(-1 * time.Minute),
			modelFilter:         "",
			expectedTotal:       0,
			expectedTokens:      0,
			expectedCost:        0.0,
			expectedStatusCount: map[string]int{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			stats, err := calculateStats(sm, tc.statusFilter, tc.sinceFilter, tc.modelFilter)
			require.NoError(t, err)

			require.Equal(t, tc.expectedTotal, stats.TotalSessions, "TotalSessions mismatch")
			require.Equal(t, tc.expectedTokens, stats.TotalTokens, "TotalTokens mismatch")
			require.InDelta(t, tc.expectedCost, stats.TotalCost, 0.0002, "TotalCost mismatch")
			require.Equal(t, len(tc.expectedStatusCount), len(stats.StatusCounts), "StatusCounts length mismatch")
			for status, count := range tc.expectedStatusCount {
				require.Equal(t, count, stats.StatusCounts[status], "StatusCount for '%s' mismatch", status)
			}
		})
	}
}
