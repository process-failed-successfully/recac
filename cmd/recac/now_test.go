package main

import (
	"bytes"
	"encoding/json"
	"os"
	"recac/internal/agent"
	"recac/internal/runner"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNowCommand(t *testing.T) {
	// Helper to create mock agent state files
	createMockStateFile := func(t *testing.T, dir, model string, tokens int) string {
		t.Helper()
		state := &agent.State{
			Model: model,
			TokenUsage: agent.TokenUsage{
				TotalPromptTokens:  tokens / 2,
				TotalResponseTokens: tokens / 2,
				TotalTokens:        tokens,
			},
		}
		content, err := json.Marshal(state)
		require.NoError(t, err)

		file, err := os.CreateTemp(dir, "agent-state-*.json")
		require.NoError(t, err)
		_, err = file.Write(content)
		require.NoError(t, err)
		file.Close()
		return file.Name()
	}

	tempDir := t.TempDir()

	// Mock Agent States. Costs are derived from running the tests and observing output.
	stateFile1 := createMockStateFile(t, tempDir, "test-model-a", 1000)            // Actual cost: $0.0010
	stateFile2 := createMockStateFile(t, tempDir, "test-model-b", 2000)            // Actual cost: $0.0020
	stateFile3 := createMockStateFile(t, tempDir, "claude-3-opus-20240229", 50000) // Actual cost: $2.2500

	now := time.Now()
	thirtyMinsAgo := now.Add(-30 * time.Minute)
	fiftyMinsAgo := now.Add(-50 * time.Minute)
	twoHoursAgo := now.Add(-2 * time.Hour)

	testCases := []struct {
		name             string
		sessions         []*runner.SessionState
		expectedContains []string
		expectedRegexp   []string
	}{
		{
			name:     "no sessions",
			sessions: []*runner.SessionState{},
			expectedContains: []string{
				"No sessions found.",
			},
		},
		{
			name: "only running sessions",
			sessions: []*runner.SessionState{
				{Name: "running-1", Status: "running", StartTime: thirtyMinsAgo, StartCommitSHA: "abcde12", AgentStateFile: stateFile1},
				{Name: "running-2", Status: "running", StartTime: fiftyMinsAgo, StartCommitSHA: "fghij34", AgentStateFile: stateFile2},
			},
			expectedContains: []string{
				"running-1", "test-model-a", "abcde12",
				"running-2", "test-model-b", "fghij34",
				"No agents completed in the last hour.",
			},
			expectedRegexp: []string{
				`Sessions Started:\s+2`,
				`Success Rate:\s+100.0%`,
				`Est. Cost:\s+\$0.0030`, // 0.0010 + 0.0020
			},
		},
		{
			name: "only recent sessions",
			sessions: []*runner.SessionState{
				{Name: "completed-recent", Status: "completed", StartTime: thirtyMinsAgo, EndTime: now, AgentStateFile: stateFile1},
				{Name: "errored-recent", Status: "error", StartTime: fiftyMinsAgo, EndTime: now.Add(-10 * time.Minute), AgentStateFile: stateFile2},
			},
			expectedContains: []string{
				"No agents are currently running.",
				"completed-recent", "completed",
				"errored-recent", "error",
			},
			expectedRegexp: []string{
				`Sessions Started:\s+2`,
				`Success Rate:\s+50.0%`,
				`Est. Cost:\s+\$0.0030`, // 0.0010 + 0.0020
			},
		},
		{
			name: "mix of all session types",
			sessions: []*runner.SessionState{
				{Name: "running-now", Status: "running", StartTime: thirtyMinsAgo, StartCommitSHA: "run123", AgentStateFile: stateFile1},
				{Name: "completed-recent", Status: "completed", StartTime: fiftyMinsAgo, EndTime: now.Add(-5 * time.Minute), AgentStateFile: stateFile2},
				{Name: "errored-recent", Status: "error", StartTime: fiftyMinsAgo, EndTime: now.Add(-10 * time.Minute)}, // No state file -> 0 cost
				{Name: "completed-old", Status: "completed", StartTime: twoHoursAgo, EndTime: twoHoursAgo.Add(10 * time.Minute), AgentStateFile: stateFile3},
				{Name: "completed-very-recent", Status: "completed", StartTime: thirtyMinsAgo, EndTime: now, AgentStateFile: stateFile3},
			},
			expectedContains: []string{
				"running-now", "run123",
				"completed-very-recent",
				"completed-recent",
				"errored-recent",
			},
			expectedRegexp: []string{
				`Sessions Started:\s+4`,
				`Success Rate:\s+66.7%`,
				`Est. Cost:\s+\$2.2530`, // 0.0010 + 0.0020 + 0 + 2.2500
			},
		},
		{
			name: "no recent activity for summary",
			sessions: []*runner.SessionState{
				{Name: "completed-old", Status: "completed", StartTime: twoHoursAgo, EndTime: twoHoursAgo.Add(time.Minute * 10)},
			},
			expectedContains: []string{
				"No agents are currently running.",
				"No agents completed in the last hour.",
			},
			expectedRegexp: []string{
				`Sessions Started:\s+0`,
				`Success Rate:\s+N/A`,
				`Est. Cost:\s+\$0.0000`,
			},
		},
	}

	// Mock loadAgentState for the duration of this test
	originalLoadAgentState := loadAgentState
	defer func() { loadAgentState = originalLoadAgentState }()
	loadAgentState = func(path string) (*agent.State, error) {
		if !strings.HasSuffix(path, ".json") {
			return nil, os.ErrNotExist
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		var state agent.State
		if err := json.Unmarshal(data, &state); err != nil {
			return nil, err
		}
		return &state, nil
	}

	originalFactory := sessionManagerFactory
	defer func() { sessionManagerFactory = originalFactory }()

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockSM := &MockSessionManager{
				Sessions: make(map[string]*runner.SessionState),
			}
			for _, s := range tc.sessions {
				mockSM.Sessions[s.Name] = s
			}
			sessionManagerFactory = func() (ISessionManager, error) {
				return mockSM, nil
			}

			rootCmd, _, _ := newRootCmd()
			nowCmd.SetOut(new(bytes.Buffer))
			rootCmd.SetArgs([]string{"now"})
			err := rootCmd.Execute()
			require.NoError(t, err)

			output := nowCmd.OutOrStdout().(*bytes.Buffer).String()

			for _, expected := range tc.expectedContains {
				require.Contains(t, output, expected)
			}

			for _, pattern := range tc.expectedRegexp {
				require.Regexp(t, pattern, output)
			}
		})
	}
}
