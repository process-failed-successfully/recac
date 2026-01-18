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

func TestPsCommandWithStaleFilter(t *testing.T) {
	sm, cleanup := setupTestSessionManager(t)
	defer cleanup()

	now := time.Now()
	// Create mock sessions with varied last activity times
	sessionActive := &runner.SessionState{Name: "session-active", Status: "completed", StartTime: now.Add(-1 * time.Hour)}
	sessionStale1d := &runner.SessionState{Name: "session-stale-1d", Status: "completed", StartTime: now.Add(-25 * time.Hour)}
	sessionStale8d := &runner.SessionState{Name: "session-stale-8d", Status: "completed", StartTime: now.Add(-9 * 24 * time.Hour)}

	// Create corresponding agent states
	createAgentState(t, sm, sessionActive, now.Add(-5*time.Minute))      // Active 5 mins ago
	createAgentState(t, sm, sessionStale1d, now.Add(-25*time.Hour))     // Stale for 1 day
	createAgentState(t, sm, sessionStale8d, now.Add(-8*24*time.Hour)) // Stale for 8 days

	require.NoError(t, sm.SaveSession(sessionActive))
	require.NoError(t, sm.SaveSession(sessionStale1d))
	require.NoError(t, sm.SaveSession(sessionStale8d))

	testCases := []struct {
		name              string
		staleValue        string
		expectError       bool
		expectedToContain []string
		expectedToOmit    []string
	}{
		{
			name:              "stale duration '12h'",
			staleValue:        "12h",
			expectedToContain: []string{"session-stale-1d", "session-stale-8d"},
			expectedToOmit:    []string{"session-active"},
		},
		{
			name:              "stale duration '7d'",
			staleValue:        "7d",
			expectedToContain: []string{"session-stale-8d"},
			expectedToOmit:    []string{"session-active", "session-stale-1d"},
		},
		{
			name:              "no sessions match",
			staleValue:        "10d",
			expectedToContain: []string{"No sessions found."},
		},
		{
			name:        "invalid stale value",
			staleValue:  "not-a-duration",
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			output, err := executeCommand(rootCmd, "ps", "--stale", tc.staleValue)

			if tc.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "invalid 'stale' value")
			} else {
				require.NoError(t, err)
				for _, expected := range tc.expectedToContain {
					assert.Contains(t, output, expected)
				}
				for _, omit := range tc.expectedToOmit {
					assert.NotContains(t, output, omit)
				}
			}
		})
	}
}

// Helper to create agent state for a session
func createAgentState(t *testing.T, sm ISessionManager, session *runner.SessionState, lastActivity time.Time) {
	t.Helper()
	agentStateFile := filepath.Join(sm.SessionsDir(), session.Name+"-agent.json")
	session.AgentStateFile = agentStateFile
	agentState := &agent.State{
		LastActivity: lastActivity,
		History:      []agent.Message{{Role: "user", Content: "Test goal for " + session.Name}},
	}
	data, err := json.Marshal(agentState)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(agentStateFile, data, 0644))
}

func TestPsCmd_NewColumns(t *testing.T) {
	tempDir := t.TempDir()
	sessionsDir := filepath.Join(tempDir, "sessions")
	require.NoError(t, os.Mkdir(sessionsDir, 0755))

	// --- Setup ---
	sm, cleanup := setupTestSessionManager(t)
	defer cleanup()

	sessionName := "test-session-goal"
	agentStateFile := filepath.Join(sm.SessionsDir(), "test-session-goal-agent.json")
	lastActivityTime := time.Now().Add(-5 * time.Minute)

	// Create a mock session
	mockSession := &runner.SessionState{
		Name:           sessionName,
		Status:         "completed",
		StartTime:      time.Now().Add(-10 * time.Minute),
		AgentStateFile: agentStateFile,
	}
	require.NoError(t, sm.SaveSession(mockSession))

	// Create a mock agent state with a goal
	mockAgentState := &agent.State{
		LastActivity: lastActivityTime,
		History: []agent.Message{
			{Role: "user", Content: "This is the goal of the session.\nThis is a second line.", Timestamp: time.Now().Add(-6 * time.Minute)},
			{Role: "assistant", Content: "I am working on it.", Timestamp: lastActivityTime},
		},
	}
	stateData, err := json.Marshal(mockAgentState)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(agentStateFile, stateData, 0644))

	// --- Execution ---
	output, err := executeCommand(rootCmd, "ps")

	// --- Assertions ---
	require.NoError(t, err)

	// Check for new headers
	assert.Contains(t, output, "LAST USED")
	assert.Contains(t, output, "GOAL")
	assert.NotContains(t, output, "STARTED") // Old header should be gone
	assert.NotContains(t, output, "DURATION") // Old header should be gone

	// Check for new column content
	assert.Contains(t, output, sessionName)
	assert.Contains(t, output, "5m ago")
	assert.Contains(t, output, "This is the goal of the session")
	assert.NotContains(t, output, "This is a second line")
}

func TestPsCommandWithResourceUsage(t *testing.T) {
	// --- Setup ---
	sm, cleanup := setupTestSessionManager(t)
	defer cleanup()

	// Create a mock session that is "running"
	// We use the current process's PID as a real, running process for gopsutil
	runningSession := &runner.SessionState{
		Name:      "test-running-session",
		Status:    "running",
		StartTime: time.Now().Add(-5 * time.Minute),
		PID:       os.Getpid(),
	}
	require.NoError(t, sm.SaveSession(runningSession))

	// Create a completed session that should not have metrics
	completedSession := &runner.SessionState{
		Name:      "test-completed-session",
		Status:    "completed",
		StartTime: time.Now().Add(-1 * time.Hour),
	}
	require.NoError(t, sm.SaveSession(completedSession))

	// --- Execution ---
	output, err := executeCommand(rootCmd, "ps")

	// --- Assertions ---
	require.NoError(t, err)

	// Check for headers
	assert.Contains(t, output, "CPU")
	assert.Contains(t, output, "MEM")

	// Split output into lines to check individual session rows
	lines := strings.Split(strings.TrimSpace(output), "\n")
	require.GreaterOrEqual(t, len(lines), 3, "Expected at least 3 lines of output (header + 2 sessions)")

	var runningLine, completedLine string
	for _, line := range lines {
		if strings.Contains(line, "test-running-session") {
			runningLine = line
		} else if strings.Contains(line, "test-completed-session") {
			completedLine = line
		}
	}
	require.NotEmpty(t, runningLine, "Running session not found in output")
	require.NotEmpty(t, completedLine, "Completed session not found in output")

	// Check the running session for metrics
	// It should have a percentage and a memory value (e.g., "0.1%", "15MB")
	assert.Regexp(t, `\d+\.\d+%`, runningLine, "Expected CPU percentage for running session")
	assert.Regexp(t, `\d+MB`, runningLine, "Expected Memory usage in MB for running session")

	// Check the completed session for "N/A"
	assert.Contains(t, completedLine, "N/A", "Expected 'N/A' for CPU of completed session")
	assert.Contains(t, completedLine, "N/A", "Expected 'N/A' for Memory of completed session")
}

func TestPsCommandWithLogs(t *testing.T) {
	// --- Setup ---
	sm, cleanup := setupTestSessionManager(t)
	defer cleanup()

	// Create a session and a corresponding log file with content
	sessionName := "session-with-logs"
	logFileName := filepath.Join(sm.SessionsDir(), sessionName+".log")
	logContent := "line 1\nline 2\nline 3\nthis is the fourth line\nand the fifth"
	err := os.WriteFile(logFileName, []byte(logContent), 0644)
	require.NoError(t, err)

	mockSession := &runner.SessionState{
		Name:      sessionName,
		Status:    "completed",
		StartTime: time.Now().Add(-1 * time.Hour),
		LogFile:   logFileName,
	}
	require.NoError(t, sm.SaveSession(mockSession))

	testCases := []struct {
		name              string
		logArg            string
		expectedToContain []string
		expectedToOmit    []string
	}{
		{
			name:              "show last 2 log lines",
			logArg:            "--logs=2",
			expectedToContain: []string{"this is the fourth line", "and the fifth"},
			expectedToOmit:    []string{"line 1", "line 2", "line 3"},
		},
		{
			name:              "show all log lines when arg is greater than lines",
			logArg:            "--logs=10",
			expectedToContain: []string{"line 1", "line 2", "line 3", "this is the fourth line", "and the fifth"},
		},
		{
			name:           "show no logs when flag is not provided",
			logArg:         "",
			expectedToOmit: []string{"line 1", "line 2", "line 3", "this is the fourth line", "and the fifth"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			args := []string{"ps"}
			if tc.logArg != "" {
				args = append(args, tc.logArg)
			}
			output, err := executeCommand(rootCmd, args...)
			require.NoError(t, err)

			for _, expected := range tc.expectedToContain {
				assert.Contains(t, output, expected)
			}
			for _, omit := range tc.expectedToOmit {
				assert.NotContains(t, output, omit)
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

func TestPsCommandWithStatusFilter(t *testing.T) {
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

	// Create mock sessions. Note: The SessionManager will transition any "running" session
	// with a non-running PID to "completed" status when ListSessions is called.
	sessionRunning := &runner.SessionState{Name: "session-running", Status: "running", StartTime: time.Now()}
	sessionCompleted := &runner.SessionState{Name: "session-completed", Status: "completed", StartTime: time.Now()}
	sessionError := &runner.SessionState{Name: "session-error", Status: "error", StartTime: time.Now()}
	sessionCompleted2 := &runner.SessionState{Name: "session-completed-2", Status: "completed", StartTime: time.Now()}

	require.NoError(t, sm.SaveSession(sessionRunning))
	require.NoError(t, sm.SaveSession(sessionCompleted))
	require.NoError(t, sm.SaveSession(sessionError))
	require.NoError(t, sm.SaveSession(sessionCompleted2))

	t.Run("filter by running (finds none)", func(t *testing.T) {
		output, err := executeCommand(rootCmd, "ps", "--status", "running")
		require.NoError(t, err)
		assert.Contains(t, output, "No sessions found.")
		assert.NotContains(t, output, "session-running")
	})

	t.Run("filter by completed (finds transitioned running session)", func(t *testing.T) {
		output, err := executeCommand(rootCmd, "ps", "--status", "completed")
		require.NoError(t, err)
		assert.Contains(t, output, "session-running") // This session is now "completed"
		assert.Contains(t, output, "session-completed")
		assert.Contains(t, output, "session-completed-2")
		assert.NotContains(t, output, "session-error")
	})

	t.Run("filter by error", func(t *testing.T) {
		output, err := executeCommand(rootCmd, "ps", "--status", "error")
		require.NoError(t, err)
		assert.NotContains(t, output, "session-running")
		assert.NotContains(t, output, "session-completed")
		assert.Contains(t, output, "session-error")
	})

	t.Run("filter with no matching sessions", func(t *testing.T) {
		output, err := executeCommand(rootCmd, "ps", "--status", "non-existent-status")
		require.NoError(t, err)
		assert.Contains(t, output, "No sessions found.")
	})

	t.Run("filter is case-insensitive", func(t *testing.T) {
		output, err := executeCommand(rootCmd, "ps", "--status", "CoMpLeTeD")
		require.NoError(t, err)
		assert.Contains(t, output, "session-running") // Also finds the transitioned session
		assert.Contains(t, output, "session-completed")
		assert.Contains(t, output, "session-completed-2")
	})
}

func TestPsCommandWithSinceFilter(t *testing.T) {
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

	now := time.Now()
	sessionRecent := &runner.SessionState{Name: "session-recent", Status: "completed", StartTime: now.Add(-5 * time.Minute)}
	sessionHourOld := &runner.SessionState{Name: "session-hour-old", Status: "completed", StartTime: now.Add(-2 * time.Hour)}
	// Use 50 hours to ensure it is safely "2 days ago" even with timezone/midnight shifts
	sessionDayOld := &runner.SessionState{Name: "session-day-old", Status: "error", StartTime: now.Add(-50 * time.Hour)}

	require.NoError(t, sm.SaveSession(sessionRecent))
	require.NoError(t, sm.SaveSession(sessionHourOld))
	require.NoError(t, sm.SaveSession(sessionDayOld))

	testCases := []struct {
		name           string
		sinceValue     string
		expectError    bool
		expectedToContain []string
		expectedToOmit  []string
	}{
		{
			name:           "relative duration '1h'",
			sinceValue:     "1h",
			expectError:    false,
			expectedToContain: []string{"session-recent"},
			expectedToOmit:  []string{"session-hour-old", "session-day-old"},
		},
		{
			name:           "relative duration '3h'",
			sinceValue:     "3h",
			expectError:    false,
			expectedToContain: []string{"session-recent", "session-hour-old"},
			expectedToOmit:  []string{"session-day-old"},
		},
		{
			name:           "absolute date",
			sinceValue:     now.Add(-90 * time.Minute).Format("2006-01-02T15:04:05Z07:00"),
			expectError:    false,
			expectedToContain: []string{"session-recent"},
			expectedToOmit:  []string{"session-hour-old", "session-day-old"},
		},
		{
			name:           "simple absolute date",
			sinceValue:     now.Add(-3 * time.Hour).Format("2006-01-02"),
			expectError:    false,
			expectedToContain: []string{"session-recent", "session-hour-old"},
			expectedToOmit:  []string{"session-day-old"},
		},
		{
			name:           "no sessions match",
			sinceValue:     "1m",
			expectError:    false,
			expectedToContain: []string{"No sessions found."},
			expectedToOmit:  []string{"session-recent", "session-hour-old", "session-day-old"},
		},
		{
			name:        "invalid since value",
			sinceValue:  "not-a-date",
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			output, err := executeCommand(rootCmd, "ps", "--since", tc.sinceValue)

			if tc.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "invalid 'since' value")
			} else {
				require.NoError(t, err)
				for _, expected := range tc.expectedToContain {
					assert.Contains(t, output, expected)
				}
				for _, omit := range tc.expectedToOmit {
					assert.NotContains(t, output, omit)
				}
			}
		})
	}
}
