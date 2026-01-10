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

	"github.com/stretchr/testify/require"
)

func TestInspectCommand(t *testing.T) {
	// Setup
	t.Setenv("RECAC_TEST", "true")
	// Create a temporary directory for session files
	tempDir := t.TempDir()
	sessionDir := filepath.Join(tempDir, ".recac", "sessions")
	err := os.MkdirAll(sessionDir, 0755)
	require.NoError(t, err)

	// Create a mock session state file
	sessionName := "test-session"
	sessionFile := filepath.Join(sessionDir, sessionName+".json")
	logFile := filepath.Join(sessionDir, sessionName+".log")
	agentStateFile := filepath.Join(sessionDir, sessionName+"_agent_state.json")

	startTime := time.Now().Add(-1 * time.Hour)
	endTime := time.Now()

	sessionState := &runner.SessionState{
		Name:           sessionName,
		Status:         "COMPLETED",
		StartTime:      startTime,
		EndTime:        endTime,
		LogFile:        logFile,
		AgentStateFile: agentStateFile,
		Error:          "mock error",
	}
	sessionData, err := json.Marshal(sessionState)
	require.NoError(t, err)
	err = os.WriteFile(sessionFile, sessionData, 0644)
	require.NoError(t, err)

	// Create a mock agent state file
	agentState := &agent.State{
		Model: "test-model",
		TokenUsage: agent.TokenUsage{
			TotalPromptTokens:   100,
			TotalResponseTokens: 200,
			TotalTokens:         300,
		},
	}
	agentData, err := json.Marshal(agentState)
	require.NoError(t, err)
	err = os.WriteFile(agentStateFile, agentData, 0644)
	require.NoError(t, err)

	// Create a mock log file
	logContent := "alpha\nbravo\ncharlie\ndelta\necho\nfoxtrot\ngolf\nhotel\nindia\njuliet\nkilo\n" // 11 lines
	err = os.WriteFile(logFile, []byte(logContent), 0644)
	require.NoError(t, err)

	// Override the sessionManagerFactory to use a mock session manager
	originalFactory := sessionManagerFactory
	sessionManagerFactory = func() (ISessionManager, error) {
		return runner.NewSessionManagerWithDir(sessionDir)
	}
	defer func() { sessionManagerFactory = originalFactory }()

	// Execute the inspect command
	output, err := executeCommand(rootCmd, "inspect", sessionName)
	require.NoError(t, err)

	// Assertions
	require.Contains(t, output, "METADATA")
	require.Regexp(t, `Name:\s+test-session`, output)
	require.Regexp(t, `Status:\s+COMPLETED`, output)
	require.Regexp(t, `Error:\s+mock error`, output)
	require.Contains(t, output, "AGENT & TOKEN STATS")
	require.Regexp(t, `Model:\s+test-model`, output)
	require.Regexp(t, `Prompt Tokens:\s+100`, output)
	require.Contains(t, output, "LOG EXCERPT (LAST 10 LINES)")
	require.Contains(t, output, "kilo")      // The last line should be present
	require.Contains(t, output, "bravo")     // The second line should be present
	require.NotContains(t, output, "alpha") // The first line should be truncated

	// Test case for a session that is still running
	runningSessionName := "running-session"
	runningSessionFile := filepath.Join(sessionDir, runningSessionName+".json")
	runningSessionState := &runner.SessionState{
		Name:      runningSessionName,
		Status:    "RUNNING",
		StartTime: startTime,
	}
	runningSessionData, err := json.Marshal(runningSessionState)
	require.NoError(t, err)
	err = os.WriteFile(runningSessionFile, runningSessionData, 0644)
	require.NoError(t, err)

	output, err = executeCommand(rootCmd, "inspect", runningSessionName)
	require.NoError(t, err)
	require.Regexp(t, `Status:\s+RUNNING`, output)
	require.True(t, strings.Contains(output, "Duration:") && !strings.Contains(output, "End Time:"))
}
