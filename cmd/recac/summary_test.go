package main

import (
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

func TestSummaryCommand(t *testing.T) {
	// Setup a temporary directory for session files
	tempDir := t.TempDir()
	t.Setenv("HOME", tempDir)

	// Create mock agent state files using a valid model name from the pricing table
	createMockAgentState(t, tempDir, "session-high-cost", "gpt-4-turbo", 10000, 20000)
	createMockAgentState(t, tempDir, "session-mid-cost", "gpt-3.5-turbo", 5000, 10000)
	createMockAgentState(t, tempDir, "session-low-cost", "gpt-3.5-turbo", 1000, 2000)
	createMockAgentState(t, tempDir, "session-running", "gpt-4-turbo", 500, 500)
	createMockAgentState(t, tempDir, "session-older", "gpt-3.5-turbo", 200, 300)
	createMockAgentState(t, tempDir, "session-oldest", "gpt-3.5-turbo", 100, 100)
	createMockAgentState(t, tempDir, "session-error", "gpt-4-turbo", 0, 0) // No cost

	now := time.Now()
	mockSessions := []*runner.SessionState{
		{Name: "session-high-cost", Status: "COMPLETED", StartTime: now.Add(-1 * time.Hour), AgentStateFile: agentStatePath(tempDir, "session-high-cost")},
		{Name: "session-mid-cost", Status: "COMPLETED", StartTime: now.Add(-2 * time.Hour), AgentStateFile: agentStatePath(tempDir, "session-mid-cost")},
		{Name: "session-low-cost", Status: "COMPLETED", StartTime: now.Add(-30 * time.Minute), AgentStateFile: agentStatePath(tempDir, "session-low-cost")},
		{Name: "session-running", Status: "RUNNING", StartTime: now.Add(-5 * time.Minute), AgentStateFile: agentStatePath(tempDir, "session-running")},
		{Name: "session-error", Status: "ERROR", StartTime: now.Add(-10 * time.Minute), AgentStateFile: agentStatePath(tempDir, "session-error")},
		{Name: "session-older", Status: "COMPLETED", StartTime: now.Add(-2 * 24 * time.Hour), AgentStateFile: agentStatePath(tempDir, "session-older")},
		{Name: "session-oldest", Status: "COMPLETED", StartTime: now.Add(-3 * 24 * time.Hour), AgentStateFile: agentStatePath(tempDir, "session-oldest")},
	}

	// Setup mock session manager
	mockSM := NewMockSessionManager()
	for _, s := range mockSessions {
		mockSM.Sessions[s.Name] = s
	}

	originalFactory := sessionManagerFactory
	sessionManagerFactory = func() (ISessionManager, error) {
		return mockSM, nil
	}
	// Restore factory at the end of the test
	defer func() {
		sessionManagerFactory = originalFactory
	}()

	// Execute the command
	rootCmd, _, _ := newRootCmd()
	output, err := executeCommand(rootCmd, "summary")
	require.NoError(t, err)

	// --- Assertions ---
	// Use Regexp to be robust against whitespace changes from tabwriter
	// Overall Stats
	require.Regexp(t, `Total Sessions:\s+7`, output)
	require.Regexp(t, `Running:\s+1`, output)
	require.Regexp(t, `Errored:\s+1`, output)

	// Cost Analysis - Now with correct model names
	// gpt-4-turbo: (10k*10 + 20k*30)/1M = 0.7
	// gpt-3.5-turbo: (5k*0.5 + 10k*1.5)/1M = 0.0175
	// gpt-3.5-turbo: (1k*0.5 + 2k*1.5)/1M = 0.0035
	// gpt-4-turbo: (0.5k*10 + 0.5k*30)/1M = 0.02
	// gpt-3.5-turbo: (0.2k*0.5 + 0.3k*1.5)/1M = 0.00055
	// gpt-3.5-turbo: (0.1k*0.5 + 0.1k*1.5)/1M = 0.0002
	// Total: 0.7 + 0.0175 + 0.0035 + 0.02 + 0.00055 + 0.0002 = 0.74175
	require.Regexp(t, `Total Estimated Cost:\s+\$0.741[78]`, output)
	require.Regexp(t, `Total Tokens:\s+49700`, output)

	// Top 5 Sessions by Cost - Assert the correct order with correct costs
	lines := strings.Split(output, "\n")
	top5, err := extractSection(lines, "TOP 5 SESSIONS BY COST", "5 MOST RECENT SESSIONS")
	require.NoError(t, err)
	expectedTop5 := []string{
		"session-high-cost", // $0.70
		"session-running",   // $0.02
		"session-mid-cost",  // $0.0175
		"session-low-cost",  // $0.0035
		"session-older",     // $0.00055
	}
	assertTableOrder(t, top5, expectedTop5)

	// 5 Most Recent Sessions - Assert the correct order
	recent5, err := extractSection(lines, "5 MOST RECENT SESSIONS", "")
	require.NoError(t, err)
	expectedRecent5 := []string{
		"session-running",
		"session-error",
		"session-low-cost",
		"session-high-cost",
		"session-mid-cost",
	}
	assertTableOrder(t, recent5, expectedRecent5)
}

// assertTableOrder checks if the lines of a table section appear in the expected order.
func assertTableOrder(t *testing.T, tableLines, expectedOrder []string) {
	// Start checking from the first data row (skip headers)
	dataLines := tableLines[3:]
	require.LessOrEqual(t, len(expectedOrder), len(dataLines), "More items expected than available in table")

	for i, expected := range expectedOrder {
		assert.Contains(t, dataLines[i], expected, "Item at index %d should be '%s'", i, expected)
	}
}

// Helper to create a mock agent state file for cost calculation
func createMockAgentState(t *testing.T, dir, sessionName, model string, prompt, response int) {
	state := agent.State{
		Model: model,
		TokenUsage: agent.TokenUsage{
			TotalPromptTokens:   prompt,
			TotalResponseTokens: response,
			TotalTokens:         prompt + response,
		},
	}
	// The cost is calculated by the pricing table, so we don't need to pass it in.
	// This makes the test more realistic as it relies on the actual CalculateCost function.

	agentStateDir := filepath.Join(dir, ".recac", "sessions")
	err := os.MkdirAll(agentStateDir, 0755)
	require.NoError(t, err)

	filePath := filepath.Join(agentStateDir, sessionName+".agent_state.json")
	file, err := os.Create(filePath)
	require.NoError(t, err)
	defer file.Close()

	err = json.NewEncoder(file).Encode(state)
	require.NoError(t, err)
}

// Helper to get the full path for a mock agent state file
func agentStatePath(dir, sessionName string) string {
	return filepath.Join(dir, ".recac", "sessions", sessionName+".agent_state.json")
}

// extractSection helps to isolate a table in the command output for more specific assertions.
func extractSection(lines []string, startMarker, endMarker string) ([]string, error) {
	var section []string
	inSection := false
	for _, line := range lines {
		if strings.Contains(line, startMarker) {
			inSection = true
		}
		if endMarker != "" && strings.Contains(line, endMarker) {
			break
		}
		if inSection {
			section = append(section, line)
		}
	}
	if len(section) == 0 {
		return nil, fmt.Errorf("could not find section starting with '%s'", startMarker)
	}
	return section, nil
}
