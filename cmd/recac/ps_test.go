package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"recac/internal/agent"
	"recac/internal/runner"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPsAndListCommands(t *testing.T) {
	tempDir := t.TempDir()
	sessionDir := filepath.Join(tempDir, "sessions")
	os.MkdirAll(sessionDir, 0755)

	oldSessionManager := sessionManagerFactory
	sessionManagerFactory = func() (ISessionManager, error) {
		return runner.NewSessionManagerWithDir(sessionDir)
	}
	defer func() { sessionManagerFactory = oldSessionManager }()

	testCases := []struct {
		name    string
		command string
	}{
		{
			name:    "ps command with no sessions",
			command: "ps",
		},
		{
			name:    "list alias with no sessions",
			command: "list",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			output, err := executeCommand(rootCmd, tc.command)
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}

			if !strings.Contains(output, "No sessions found.") {
				t.Errorf("expected output to contain 'No sessions found.', got '%s'", output)
			}
		})
	}
}

func TestPsCommandWithCosts(t *testing.T) {
	tempDir := t.TempDir()
	sessionsDir := filepath.Join(tempDir, "sessions")
	require.NoError(t, os.Mkdir(sessionsDir, 0755))

	originalFactory := sessionManagerFactory
	sessionManagerFactory = func() (ISessionManager, error) {
		return runner.NewSessionManagerWithDir(sessionsDir)
	}
	defer func() { sessionManagerFactory = originalFactory }()

	sm, err := sessionManagerFactory()
	require.NoError(t, err)

	sessionWithCost := &runner.SessionState{
		Name:           "session-with-cost",
		Status:         "completed",
		StartTime:      time.Now().Add(-1 * time.Hour),
		EndTime:        time.Now(),
		AgentStateFile: filepath.Join(sessionsDir, "session-with-cost-agent-state.json"),
	}
	sessionWithoutCost := &runner.SessionState{
		Name:           "session-without-cost",
		Status:         "running",
		StartTime:      time.Now().Add(-10 * time.Minute),
		AgentStateFile: filepath.Join(sessionsDir, "non-existent-agent-state.json"),
	}

	agentState := &agent.State{
		Model: "gemini-pro",
		TokenUsage: agent.TokenUsage{
			TotalPromptTokens:   1000,
			TotalResponseTokens: 2000,
			TotalTokens:         3000,
		},
	}
	stateData, err := json.Marshal(agentState)
	require.NoError(t, err)
	err = os.WriteFile(sessionWithCost.AgentStateFile, stateData, 0644)
	require.NoError(t, err)

	err = sm.SaveSession(sessionWithCost)
	require.NoError(t, err)
	err = sm.SaveSession(sessionWithoutCost)
	require.NoError(t, err)

	output, err := executeCommand(rootCmd, "ps", "--costs")
	require.NoError(t, err)

	assert.Contains(t, output, "NAME")
	assert.Contains(t, output, "STATUS")
	assert.Contains(t, output, "COST")
	assert.Contains(t, output, "TOTAL_TOKENS")

	assert.Regexp(t, `session-with-cost\s+completed`, output)
	assert.Contains(t, output, "1000")
	assert.Contains(t, output, "2000")
	assert.Contains(t, output, "3000")

	assert.Regexp(t, `session-without-cost\s+completed`, output)
	assert.Contains(t, output, "N/A")
}

func TestPsCmdSort(t *testing.T) {
	tempDir := t.TempDir()
	sessionsDir := filepath.Join(tempDir, "sessions")
	require.NoError(t, os.Mkdir(sessionsDir, 0755))

	originalFactory := sessionManagerFactory
	sessionManagerFactory = func() (ISessionManager, error) {
		return runner.NewSessionManagerWithDir(sessionsDir)
	}
	defer func() { sessionManagerFactory = originalFactory }()

	sm, err := sessionManagerFactory()
	require.NoError(t, err)

	// Create mock sessions
	sessionA := &runner.SessionState{
		Name:           "session-a",
		Status:         "completed",
		StartTime:      time.Now().Add(-2 * time.Hour), // Older
		AgentStateFile: filepath.Join(sessionsDir, "session-a-agent-state.json"),
	}
	sessionB := &runner.SessionState{
		Name:           "session-b",
		Status:         "completed",
		StartTime:      time.Now().Add(-1 * time.Hour), // Newer
		AgentStateFile: filepath.Join(sessionsDir, "session-b-agent-state.json"),
	}
	sessionC := &runner.SessionState{
		Name:           "session-c",
		Status:         "running",
		StartTime:      time.Now().Add(-3 * time.Hour), // Oldest
		AgentStateFile: filepath.Join(sessionsDir, "session-c-agent-state.json"),
	}

	// Create mock agent states with different token counts for cost calculation
	agentStateA := &agent.State{Model: "gemini-pro", TokenUsage: agent.TokenUsage{TotalPromptTokens: 500, TotalResponseTokens: 500, TotalTokens: 1000}}    // Low cost
	agentStateB := &agent.State{Model: "gemini-pro", TokenUsage: agent.TokenUsage{TotalPromptTokens: 1500, TotalResponseTokens: 1500, TotalTokens: 3000}} // High cost
	agentStateC := &agent.State{Model: "gemini-pro", TokenUsage: agent.TokenUsage{TotalPromptTokens: 1000, TotalResponseTokens: 1000, TotalTokens: 2000}} // Medium cost

	// Write agent state files
	dataA, _ := json.Marshal(agentStateA)
	os.WriteFile(sessionA.AgentStateFile, dataA, 0644)
	dataB, _ := json.Marshal(agentStateB)
	os.WriteFile(sessionB.AgentStateFile, dataB, 0644)
	dataC, _ := json.Marshal(agentStateC)
	os.WriteFile(sessionC.AgentStateFile, dataC, 0644)

	// Save sessions
	require.NoError(t, sm.SaveSession(sessionA))
	require.NoError(t, sm.SaveSession(sessionB))
	require.NoError(t, sm.SaveSession(sessionC))

	// --- Test Cases ---

	t.Run("sort by name", func(t *testing.T) {
		output, err := executeCommand(rootCmd, "ps", "--sort", "name")
		require.NoError(t, err)
		assert.Regexp(t, `(?s)session-a.*session-b.*session-c`, output)
	})

	t.Run("sort by time (default)", func(t *testing.T) {
		output, err := executeCommand(rootCmd, "ps")
		require.NoError(t, err)
		// Expected: session-b (newest), session-a, session-c (oldest)
		assert.Regexp(t, `(?s)session-b.*session-a.*session-c`, output)
	})

	t.Run("sort by cost", func(t *testing.T) {
		output, err := executeCommand(rootCmd, "ps", "--sort", "cost", "--costs")
		require.NoError(t, err)
		// Expected: session-b (highest cost), session-c, session-a (lowest cost)
		assert.Regexp(t, `(?s)session-b.*session-c.*session-a`, output)
	})
}

func TestPsCommandDetailView(t *testing.T) {
	tempDir := t.TempDir()
	sessionsDir := filepath.Join(tempDir, "sessions")
	require.NoError(t, os.Mkdir(sessionsDir, 0755))

	originalFactory := sessionManagerFactory
	sessionManagerFactory = func() (ISessionManager, error) {
		return runner.NewSessionManagerWithDir(sessionsDir)
	}
	defer func() { sessionManagerFactory = originalFactory }()

	sm, err := sessionManagerFactory()
	require.NoError(t, err)

	// Create a mock session
	sessionName := "test-session-detail"
	logFile := filepath.Join(sessionsDir, sessionName+".log")
	err = os.WriteFile(logFile, []byte("line 1\nline 2\n"), 0644)
	require.NoError(t, err)

	agentStateFile := filepath.Join(sessionsDir, sessionName+"-agent-state.json")
	agentState := &agent.State{
		Model: "gemini-pro",
		TokenUsage: agent.TokenUsage{
			TotalPromptTokens:   50,
			TotalResponseTokens: 150,
			TotalTokens:         200,
		},
	}
	stateData, err := json.Marshal(agentState)
	require.NoError(t, err)
	err = os.WriteFile(agentStateFile, stateData, 0644)
	require.NoError(t, err)

	session := &runner.SessionState{
		Name:           sessionName,
		Status:         "completed",
		StartTime:      time.Now().Add(-1 * time.Hour),
		EndTime:        time.Now(),
		LogFile:        logFile,
		AgentStateFile: agentStateFile,
	}
	err = sm.SaveSession(session)
	require.NoError(t, err)

	// Execute the "ps <session-name>" command
	output, err := executeCommand(rootCmd, "ps", sessionName)
	require.NoError(t, err)

	// Assert that the output contains the detailed view information using regular expressions
	assert.Regexp(t, `Session Details`, output)
	assert.Regexp(t, `Name:\s+test-session-detail`, output)
	assert.Regexp(t, `Status:\s+completed`, output)
	assert.Regexp(t, `Total Tokens:\s+200`, output)
	assert.Regexp(t, `Cost:`, output)
	assert.Regexp(t, `Recent Logs`, output)
	assert.Contains(t, output, "line 1")
	assert.Contains(t, output, "line 2")
}
