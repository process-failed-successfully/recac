package main

import (
	"encoding/json"
	"io"
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

func TestDisplayStats(t *testing.T) {
	stats := &AggregateStats{
		TotalSessions:       10,
		TotalTokens:         1000,
		TotalPromptTokens:   300,
		TotalResponseTokens: 700,
		TotalCost:           1.2345,
		StatusCounts: map[string]int{
			"completed": 7,
			"failed":    3,
		},
	}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Restore stdout when done
	defer func() {
		os.Stdout = oldStdout
	}()

	displayStats(stats)

	w.Close()
	out, _ := io.ReadAll(r)
	output := string(out)

	require.Contains(t, output, "AGGREGATE SESSION STATISTICS")
	require.Contains(t, output, "Total Sessions:")
	require.Contains(t, output, "10")
	require.Contains(t, output, "Total Tokens:")
	require.Contains(t, output, "1000")
	require.Contains(t, output, "Total Estimated Cost:")
	require.Contains(t, output, "1.2345")
	require.Contains(t, output, "completed:")
	require.Contains(t, output, "7")
	require.Contains(t, output, "failed:")
	require.Contains(t, output, "3")
}
