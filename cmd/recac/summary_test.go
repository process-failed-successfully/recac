package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"recac/internal/agent"
	"recac/internal/runner"

	"github.com/stretchr/testify/require"
)

// Helper to create a temporary agent state file and return its path
func createTempAgentState(t *testing.T, dir string, state agent.State) string {
	t.Helper()
	file, err := os.CreateTemp(dir, "agent-state-*.json")
	require.NoError(t, err)
	defer file.Close()

	data, err := json.Marshal(state)
	require.NoError(t, err)

	_, err = file.Write(data)
	require.NoError(t, err)

	return file.Name()
}

func TestSummaryCommand(t *testing.T) {
	tempDir := t.TempDir()

	mockSessions := []*runner.SessionState{
		{
			Name:      "running-session-1",
			Status:    "running",
			StartTime: time.Now().Add(-10 * time.Minute),
			AgentStateFile: createTempAgentState(t, tempDir, agent.State{
				Model: "gpt-4o",
				TokenUsage: agent.TokenUsage{TotalPromptTokens: 100, TotalResponseTokens: 200, TotalTokens: 300},
			}),
		},
		{
			Name:      "completed-session-2",
			Status:    "completed",
			StartTime: time.Now().Add(-30 * time.Minute),
			AgentStateFile: createTempAgentState(t, tempDir, agent.State{
				Model: "gemini-pro",
				TokenUsage: agent.TokenUsage{TotalPromptTokens: 500, TotalResponseTokens: 1000, TotalTokens: 1500},
			}),
		},
		{
			Name:      "errored-session-3",
			Status:    "error",
			StartTime: time.Now().Add(-60 * time.Minute),
			AgentStateFile: createTempAgentState(t, tempDir, agent.State{
				Model: "gpt-4o",
				TokenUsage: agent.TokenUsage{TotalPromptTokens: 50, TotalResponseTokens: 50, TotalTokens: 100},
			}),
		},
		{
			Name:           "completed-no-cost-4",
			Status:         "completed",
			StartTime:      time.Now().Add(-2 * time.Hour),
			AgentStateFile: "", // No agent state file, should result in $0.00 cost
		},
		{
			Name:      "ancient-session-5",
			Status:    "completed",
			StartTime: time.Now().Add(-24 * time.Hour),
			AgentStateFile: createTempAgentState(t, tempDir, agent.State{
				Model: "gemini-pro",
				TokenUsage: agent.TokenUsage{TotalPromptTokens: 2000, TotalResponseTokens: 4000, TotalTokens: 6000},
			}),
		},
		{
			Name:      "most-expensive-6",
			Status:    "completed",
			StartTime: time.Now().Add(-5 * time.Minute),
			AgentStateFile: createTempAgentState(t, tempDir, agent.State{
				Model: "claude-3-opus-20240229", // A more expensive model
				TokenUsage: agent.TokenUsage{TotalPromptTokens: 10000, TotalResponseTokens: 20000, TotalTokens: 30000},
			}),
		},
	}

	// Create a mock session manager with the defined sessions
	mockSessionsMap := make(map[string]*runner.SessionState)
	for _, s := range mockSessions {
		mockSessionsMap[s.Name] = s
	}

	sm := &MockSessionManager{
		Sessions: mockSessionsMap,
		SessionsDirFunc: func() string {
			return filepath.Join(tempDir, "sessions")
		},
	}

	// Override the factory to return our mock session manager
	originalFactory := sessionManagerFactory
	sessionManagerFactory = func() (ISessionManager, error) {
		return sm, nil
	}
	defer func() { sessionManagerFactory = originalFactory }()

	// Execute the summary command
	rootCmd, _, _ := newRootCmd()
	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)
	rootCmd.SetArgs([]string{"summary"})
	err := rootCmd.Execute()
	require.NoError(t, err)

	output := out.String()

	// --- Assertions for the output ---

	// Overview section
	require.Contains(t, output, "==> Overview")
	require.Regexp(t, `Total Sessions:\s+6`, output)
	require.Regexp(t, `Running:\s+1`, output)
	require.Regexp(t, `Completed:\s+4`, output)
	require.Regexp(t, `Errored:\s+1`, output)

	// Usage Stats section
	require.Contains(t, output, "==> Usage Stats (All Time)")
	require.Regexp(t, `Total Cost:\s+\$1.66`, output) // 1.50 (claude) + 0.009 (gemini) + 0.003 (gemini) + 0.005 (gpt4) + 0.001 (gpt4) = ~1.518 -> rounded
	require.Regexp(t, `Total Tokens:\s+37900`, output)

	// Split output into sections for more precise testing
	sections := strings.Split(output, "==>")
	require.Len(t, sections, 5, "Should have 5 sections: an empty one, Overview, Usage, Recent, Most Expensive")

	overviewSection := sections[1]
	usageSection := sections[2]
	recentSection := sections[3]
	expensiveSection := sections[4]

	// Overview section assertions
	require.Regexp(t, `Total Sessions:\s+6`, overviewSection)
	require.Regexp(t, `Running:\s+1`, overviewSection)
	require.Regexp(t, `Completed:\s+4`, overviewSection)
	require.Regexp(t, `Errored:\s+1`, overviewSection)

	// Usage Stats section assertions
	require.Regexp(t, `Total Cost:\s+\$1.66`, usageSection)
	require.Regexp(t, `Total Tokens:\s+37900`, usageSection)

	// Recent Sessions section assertions
	require.Contains(t, recentSection, "Recent Sessions (Last 5)")
	require.Contains(t, recentSection, "most-expensive-6")
	require.Contains(t, recentSection, "running-session-1")
	require.Contains(t, recentSection, "completed-session-2")
	require.Contains(t, recentSection, "errored-session-3")
	require.Contains(t, recentSection, "completed-no-cost-4")
	require.NotContains(t, recentSection, "ancient-session-5", "ancient-session-5 should be excluded from recent 5")
	require.Regexp(t, `most-expensive-6\s+completed\s+5m\s+\$1.65`, recentSection)

	// Most Expensive Sessions section assertions
	require.Contains(t, expensiveSection, "Most Expensive Sessions (Top 3)")
	require.Contains(t, expensiveSection, "most-expensive-6")
	require.Contains(t, expensiveSection, "ancient-session-5", "ancient-session-5 should be in the most expensive list")
	require.Contains(t, expensiveSection, "running-session-1", "running-session-1 should be in the top 3 expensive")
	require.NotContains(t, expensiveSection, "completed-session-2", "completed-session-2 should NOT be in the top 3 expensive")
	require.Regexp(t, `most-expensive-6\s+completed\s+\$1.65`, expensiveSection)
	require.Regexp(t, `ancient-session-5\s+completed\s+\$0.01`, expensiveSection)
}

func TestSummaryCommand_NoSessions(t *testing.T) {
	sm := &MockSessionManager{
		Sessions: make(map[string]*runner.SessionState),
	}

	// Override the factory
	originalFactory := sessionManagerFactory
	sessionManagerFactory = func() (ISessionManager, error) {
		return sm, nil
	}
	defer func() { sessionManagerFactory = originalFactory }()

	// Execute the command
	rootCmd, _, _ := newRootCmd()
	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)
	rootCmd.SetArgs([]string{"summary"})
	err := rootCmd.Execute()
	require.NoError(t, err)

	// Assert that a user-friendly message is printed
	require.Contains(t, out.String(), "No sessions to display.")
}
