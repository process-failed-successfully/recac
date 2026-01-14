package main

import (
	"os"
	"path/filepath"
	"recac/internal/runner"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestTopCmdDataFetching(t *testing.T) {
	// Setup: Create a temporary session manager and a fake running session
	sm, smCleanup := setupTestSessionManager(t)
	defer smCleanup()

	// Use the current process's PID as a stand-in for a real agent process
	// This ensures gopsutil can find and read metrics for the process.
	pid := os.Getpid()
	sessionName := "test-running-session"

	// Manually create a session state file
	sessionState := &runner.SessionState{
		Name:      sessionName,
		Status:    "running",
		PID:       pid,
		StartTime: time.Now(),
		LogFile:   filepath.Join(sm.GetBaseDir(), "sessions", "test.log"),
		AgentStateFile: filepath.Join(sm.GetBaseDir(), "sessions", "agent.json"),
	}
	err := sm.SaveSession(sessionState)
	require.NoError(t, err)

	// Another session that is completed, to test filtering
	completedSession := &runner.SessionState{
		Name:      "test-completed-session",
		Status:    "completed",
		PID:       0, // No PID
		StartTime: time.Now().Add(-1 * time.Hour),
		EndTime:   time.Now(),
	}
	err = sm.SaveSession(completedSession)
	require.NoError(t, err)

	// Execute the command's refresh logic.
	// We don't need to monkey patch the factory because setupTestSessionManager already does.
	refreshCmd := refreshTopCmd()
	msg := refreshCmd()

	// Assert the message is the correct type and contains the expected data
	refreshedMsg, ok := msg.(sessionsRefreshedMsg)
	require.True(t, ok, "Expected a sessionsRefreshedMsg")

	// It should only find the one "running" session
	require.Len(t, refreshedMsg.sessions, 1, "Should have found exactly one running session")
	require.Equal(t, sessionName, refreshedMsg.sessions[0].Name)

	// It should contain metrics for the running session
	require.Contains(t, refreshedMsg.metrics, sessionName)
	metric := refreshedMsg.metrics[sessionName]
	require.Equal(t, int32(pid), metric.PID)

	// CPU and Mem might be 0 on the first tick, so we just check they are not negative
	require.GreaterOrEqual(t, metric.CPUPercent, 0.0)
	require.GreaterOrEqual(t, metric.MemPercent, float32(0.0))
}
