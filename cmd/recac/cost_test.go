package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"recac/internal/agent"
	"recac/internal/runner"
	"recac/internal/ui"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCostCommand_Static(t *testing.T) {
	// Create a temporary directory for mock session and agent state files
	tempDir := t.TempDir()
	sessionsDir := filepath.Join(tempDir, "sessions")
	err := os.Mkdir(sessionsDir, 0755)
	require.NoError(t, err)

	// --- Create Mock Data ---

	// Session 1: High-cost gemini-1.5-pro
	session1State := &runner.SessionState{
		Name:           "test-session-1",
		Status:         "COMPLETED",
		StartTime:      time.Now().Add(-2 * time.Hour),
		EndTime:        time.Now().Add(-1 * time.Hour),
		AgentStateFile: filepath.Join(sessionsDir, "agent_state_1.json"),
	}
	agent1State := &agent.State{
		Model: "gemini-1.5-pro-latest",
		TokenUsage: agent.TokenUsage{
			TotalPromptTokens:   200000,
			TotalResponseTokens: 50000,
			TotalTokens:         250000,
		},
	}

	// Session 2: Medium-cost gpt-4
	session2State := &runner.SessionState{
		Name:           "test-session-2",
		Status:         "COMPLETED",
		StartTime:      time.Now().Add(-3 * time.Hour),
		EndTime:        time.Now().Add(-2 * time.Hour),
		AgentStateFile: filepath.Join(sessionsDir, "agent_state_2.json"),
	}
	agent2State := &agent.State{
		Model: "gpt-4-turbo",
		TokenUsage: agent.TokenUsage{
			TotalPromptTokens:   10000,
			TotalResponseTokens: 30000,
			TotalTokens:         40000,
		},
	}

	// Session 3: Low-cost gpt-4 (again, to test aggregation)
	session3State := &runner.SessionState{
		Name:           "test-session-3",
		Status:         "COMPLETED",
		StartTime:      time.Now().Add(-4 * time.Hour),
		EndTime:        time.Now().Add(-3 * time.Hour),
		AgentStateFile: filepath.Join(sessionsDir, "agent_state_3.json"),
	}
	agent3State := &agent.State{
		Model: "gpt-4-turbo",
		TokenUsage: agent.TokenUsage{
			TotalPromptTokens:   5000,
			TotalResponseTokens: 10000,
			TotalTokens:         15000,
		},
	}

	// Session 4: No agent state file (should be skipped)
	session4State := &runner.SessionState{
		Name:           "test-session-4-no-state",
		Status:         "RUNNING",
		StartTime:      time.Now().Add(-1 * time.Hour),
		AgentStateFile: "", // No state file
	}

	// Write mock files
	mockSessions := []*runner.SessionState{session1State, session2State, session3State, session4State}
	mockAgentStates := []*agent.State{agent1State, agent2State, agent3State}
	for i, session := range mockSessions {
		sessionBytes, err := json.Marshal(session)
		require.NoError(t, err)
		sessionFilePath := filepath.Join(sessionsDir, fmt.Sprintf("%s.json", session.Name))
		err = os.WriteFile(sessionFilePath, sessionBytes, 0644)
		require.NoError(t, err)
		if i < len(mockAgentStates) {
			agentBytes, err := json.Marshal(mockAgentStates[i])
			require.NoError(t, err)
			err = os.WriteFile(session.AgentStateFile, agentBytes, 0644)
			require.NoError(t, err)
		}
	}

	// --- Execute Command ---

	// Mock the session manager factory
	originalFactory := sessionManagerFactory
	sessionManagerFactory = func() (ISessionManager, error) {
		return runner.NewSessionManagerWithDir(sessionsDir)
	}
	defer func() { sessionManagerFactory = originalFactory }()

	// Execute the command and capture output
	rootCmd, _, _ := newRootCmd()
	output, err := executeCommand(rootCmd, "cost", "--limit", "2")
	require.NoError(t, err)

	// --- Assertions ---

	// Check for correct headers
	require.Contains(t, output, "COST BY MODEL")
	require.Contains(t, output, "TOP SESSIONS BY COST")
	require.Contains(t, output, "TOTALS")

	// Check model aggregation (gemini-1.5-pro-latest has the highest total cost)
	// Using regex to be robust against whitespace changes from tabwriter
	require.Regexp(t, `gemini-1.5-pro-latest\s+\$2.4500`, output)
	require.Regexp(t, `gpt-4-turbo\s+\$1.3500`, output)

	// Check top sessions by cost (limit is 2, so session-1 and session-2 should be listed)
	require.Regexp(t, `test-session-1\s+gemini-1.5-pro-latest\s+\$2.450000`, output)
	require.Regexp(t, `test-session-2\s+gpt-4-turbo\s+\$1.000000`, output)
	require.NotContains(t, output, "test-session-3") // Excluded by --limit=2

	// Check totals
	require.Contains(t, output, "Total Estimated Cost:")
	require.Regexp(t, `\$3.8000`, output)
	require.Contains(t, output, "Total Tokens:")
	require.Regexp(t, `305000`, output)

	// Verify that the session with no agent state was ignored and didn't cause a panic
	require.NotContains(t, output, "test-session-4-no-state")
}

func TestCostCommand_WatchFlag(t *testing.T) {
	// --- Setup ---
	tempDir := t.TempDir()
	sessionsDir := filepath.Join(tempDir, "sessions")
	err := os.Mkdir(sessionsDir, 0755)
	require.NoError(t, err)

	// Mock the session manager factory
	var createdSm ISessionManager
	originalFactory := sessionManagerFactory
	sessionManagerFactory = func() (ISessionManager, error) {
		sm, err := runner.NewSessionManagerWithDir(sessionsDir)
		createdSm = sm // Capture the instance for assertion
		return sm, err
	}
	defer func() { sessionManagerFactory = originalFactory }()

	// Mock the TUI starter function
	var tuiStarted bool
	var receivedSm ui.SessionManager

	// Temporarily replace the TUI starter with our mock
	originalTUIStarter := ui.StartCostTUI
	ui.SetStartCostTUIForTest(func(sm ui.SessionManager) error {
		tuiStarted = true
		receivedSm = sm
		return nil
	})
	// Restore the original function after the test
	defer ui.SetStartCostTUIForTest(originalTUIStarter)


	// --- Execute ---
	rootCmd, _, _ := newRootCmd()
	_, err = executeCommand(rootCmd, "cost", "--watch")
	require.NoError(t, err)

	// --- Assertions ---
	assert.True(t, tuiStarted, "ui.StartCostTUI should have been called")
	assert.NotNil(t, receivedSm, "TUI should have received a non-nil session manager")
	assert.Equal(t, createdSm, receivedSm, "The session manager passed to the TUI should be the one created by the factory")
	assert.NotNil(t, ui.LoadAgentState, "The LoadAgentState function should have been injected into the ui package")
}

func TestCostCommand_Flags(t *testing.T) {
	cmd := costCmd
	flag := cmd.Flags().Lookup("watch")
	require.NotNil(t, flag, "the --watch flag should be registered")
	require.Equal(t, "false", flag.DefValue, "the --watch flag should default to false")
	require.Equal(t, "Launch a real-time TUI to monitor session costs", flag.Usage, "the --watch flag should have the correct usage message")
}
