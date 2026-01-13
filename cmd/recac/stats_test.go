package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"recac/internal/agent"
	"recac/internal/runner"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStatsCommandWithFilters(t *testing.T) {
	// --- Test Setup ---
	tempDir := t.TempDir()
	t.Setenv("HOME", tempDir) // Isolate session manager

	mockSessions := createMockSessionsForStats(t, tempDir)
	mockSM := &MockSessionManager{
		Sessions: make(map[string]*runner.SessionState),
	}
	for _, s := range mockSessions {
		mockSM.Sessions[s.Name] = s
	}

	// Override the factory to inject our mock
	originalFactory := sessionManagerFactory
	sessionManagerFactory = func() (ISessionManager, error) {
		return mockSM, nil
	}
	defer func() { sessionManagerFactory = originalFactory }()

	// --- Test Cases ---
	testCases := []struct {
		name           string
		args           []string
		expectedCount  int
		expectedCost   float64
		expectedTokens int
		expectedStatus map[string]int
	}{
		{
			name:           "No filters",
			args:           []string{},
			expectedCount:  4,
			expectedCost:   0.0243, // 0.016 + 0.008 + 0.0003
			expectedTokens: 1500,
			expectedStatus: map[string]int{"completed": 2, "error": 1, "running": 1},
		},
		{
			name:           "Filter by status 'completed'",
			args:           []string{"--status", "completed"},
			expectedCount:  2,
			expectedCost:   0.024, // 0.016 + 0.008
			expectedTokens: 1200,
			expectedStatus: map[string]int{"completed": 2},
		},
		{
			name:           "Filter by status 'error'",
			args:           []string{"--status", "error"},
			expectedCount:  1,
			expectedCost:   0.0003,
			expectedTokens: 300,
			expectedStatus: map[string]int{"error": 1},
		},
		{
			name:           "Filter by name 'PROJ-123'",
			args:           []string{"--name", "PROJ-123"},
			expectedCount:  1,
			expectedCost:   0.0003,
			expectedTokens: 300,
			expectedStatus: map[string]int{"error": 1},
		},
		{
			name:           "Filter by name 'feature'",
			args:           []string{"--name", "feature"},
			expectedCount:  2,
			expectedCost:   0.024,
			expectedTokens: 1200,
			expectedStatus: map[string]int{"completed": 2},
		},
		{
			name:           "Filter by since '2023-10-27T11:00:00Z'",
			args:           []string{"--since", "2023-10-27T11:00:00Z"},
			expectedCount:  2,
			expectedCost:   0.0003, // PROJ-123 (error) + running-task (no cost)
			expectedTokens: 300,
			expectedStatus: map[string]int{"error": 1, "running": 1},
		},
		{
			name:           "Filter by until '2023-10-27T09:00:00Z'",
			args:           []string{"--until", "2023-10-27T09:00:00Z"},
			expectedCount:  1,
			expectedCost:   0.016,
			expectedTokens: 800,
			expectedStatus: map[string]int{"completed": 1},
		},
		{
			name:           "Filter by since and until (time range)",
			args:           []string{"--since", "2023-10-27T09:30:00Z", "--until", "2023-10-27T10:30:00Z"},
			expectedCount:  1,
			expectedCost:   0.008,
			expectedTokens: 400,
			expectedStatus: map[string]int{"completed": 1},
		},
		{
			name:           "Filter by name and status",
			args:           []string{"--name", "feature", "--status", "completed"},
			expectedCount:  2,
			expectedCost:   0.024,
			expectedTokens: 1200,
			expectedStatus: map[string]int{"completed": 2},
		},
		{
			name:           "Filter with no matches",
			args:           []string{"--name", "non-existent"},
			expectedCount:  0,
			expectedCost:   0.0,
			expectedTokens: 0,
			expectedStatus: map[string]int{},
		},
		{
			name:           "Invalid since format",
			args:           []string{"--since", "not-a-date"},
			expectedCount:  -1, // Indicates error expected
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			rootCmd, _, _ := newRootCmd()
			statsCmd, _, err := rootCmd.Find([]string{"stats"})
			require.NoError(t, err)

			// Reset flags before each run
			statsCmd.Flags().Set("status", "")
			statsCmd.Flags().Set("name", "")
			statsCmd.Flags().Set("since", "")
			statsCmd.Flags().Set("until", "")

			// Set new flags for the test case
			err = statsCmd.ParseFlags(tc.args)
			require.NoError(t, err)

			outBuf := new(bytes.Buffer)
			statsCmd.SetOut(outBuf)
			errBuf := new(bytes.Buffer)
			statsCmd.SetErr(errBuf)

			err = statsCmd.Execute()

			if tc.expectedCount == -1 {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "invalid --since time format")
				return
			}

			require.NoError(t, err, "Command execution failed: %v", err)

			// Custom parsing because we can't easily get the struct back
			output := outBuf.String()
			stats := parseStatsOutput(output)

			assert.Equal(t, tc.expectedCount, stats.TotalSessions, "Mismatched session count")
			assert.InDelta(t, tc.expectedCost, stats.TotalCost, 0.000001, "Mismatched total cost")
			assert.Equal(t, tc.expectedTokens, stats.TotalTokens, "Mismatched total tokens")
			assert.Equal(t, tc.expectedStatus, stats.StatusCounts, "Mismatched status breakdown")
		})
	}
}

// createMockSessionsForStats creates a slice of sessions and their agent state files.
func createMockSessionsForStats(t *testing.T, baseDir string) []*runner.SessionState {
	sessionsDir := filepath.Join(baseDir, ".recac", "sessions")
	require.NoError(t, os.MkdirAll(sessionsDir, 0755))

	createStateFile := func(name string, tokens int, model string) string {
		state := agent.State{
			Model: model,
			TokenUsage: agent.TokenUsage{
				TotalTokens:         tokens,
				TotalPromptTokens:   tokens / 2,
				TotalResponseTokens: tokens / 2,
			},
		}
		content, err := json.Marshal(state)
		require.NoError(t, err)
		filePath := filepath.Join(sessionsDir, fmt.Sprintf("%s_agent_state.json", name))
		require.NoError(t, os.WriteFile(filePath, content, 0644))
		return filePath
	}

	sessions := []*runner.SessionState{
		{
			Name:           "feature-login",
			Status:         "completed",
			StartTime:      time.Date(2023, 10, 27, 8, 0, 0, 0, time.UTC),
			AgentStateFile: createStateFile("feature-login", 800, "gpt-4-turbo"),
		},
		{
			Name:           "feature-logout",
			Status:         "completed",
			StartTime:      time.Date(2023, 10, 27, 10, 0, 0, 0, time.UTC),
			AgentStateFile: createStateFile("feature-logout", 400, "gpt-4-turbo"),
		},
		{
			Name:           "PROJ-123-bugfix",
			Status:         "error",
			StartTime:      time.Date(2023, 10, 27, 12, 0, 0, 0, time.UTC),
			AgentStateFile: createStateFile("PROJ-123-bugfix", 300, "gemini-pro"),
		},
		{
			Name:           "running-task",
			Status:         "running",
			StartTime:      time.Date(2023, 10, 27, 14, 0, 0, 0, time.UTC),
			AgentStateFile: "", // No agent state yet
		},
	}
	return sessions
}

// parseStatsOutput is a helper to parse the CLI output back into a struct for testing.
func parseStatsOutput(output string) AggregateStats {
	stats := AggregateStats{
		StatusCounts: make(map[string]int),
	}
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		parts := strings.Split(line, ":\t")
		if len(parts) < 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		switch key {
		case "Matching Sessions":
			fmt.Sscanf(value, "%d", &stats.TotalSessions)
		case "Total Tokens":
			fmt.Sscanf(value, "%d", &stats.TotalTokens)
		case "Total Estimated Cost":
			fmt.Sscanf(value, "$%f", &stats.TotalCost)
		case "completed", "error", "running":
			var count int
			fmt.Sscanf(value, "%d", &count)
			stats.StatusCounts[key] = count
		}
	}
	return stats
}
