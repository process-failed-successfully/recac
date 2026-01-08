package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"recac/internal/agent"
	"recac/internal/runner"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRunHistoryCmd(t *testing.T) {
	// Create a temporary directory for all test artifacts
	baseTempDir, err := os.MkdirTemp("", "history-test-base")
	assert.NoError(t, err)
	defer os.RemoveAll(baseTempDir)

	sessionsDir := filepath.Join(baseTempDir, "sessions")
	assert.NoError(t, os.Mkdir(sessionsDir, 0755))

	// Create a mock agent state file in a separate location
	agentStateDir := filepath.Join(baseTempDir, "workspace")
	assert.NoError(t, os.Mkdir(agentStateDir, 0755))
	agentStateFile := filepath.Join(agentStateDir, ".agent_state.json")

	agentState := agent.State{
		TokenUsage: agent.TokenUsage{
			TotalTokens: 12345,
		},
	}
	agentStateData, err := json.Marshal(agentState)
	assert.NoError(t, err)
	err = os.WriteFile(agentStateFile, agentStateData, 0644)
	assert.NoError(t, err)

	// --- Test Case 1: No Completed Sessions ---
	t.Run("NoCompletedSessions", func(t *testing.T) {
		// Redirect stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		mockFactory := func() (*runner.SessionManager, error) {
			// Point to an empty sessions dir
			emptyDir := t.TempDir()
			return runner.NewSessionManagerWithDir(emptyDir)
		}

		err := runHistoryCmd(mockFactory)
		assert.NoError(t, err)

		// Restore stdout and capture output
		w.Close()
		os.Stdout = oldStdout
		var buf bytes.Buffer
		buf.ReadFrom(r)

		assert.Equal(t, "No completed sessions found.\n", buf.String())
	})

	// --- Test Case 2: One Completed Session ---
	t.Run("OneCompletedSession", func(t *testing.T) {
		// Setup: Create a session file
		session := &runner.SessionState{
			Name:           "test-session-1",
			Status:         "completed",
			StartTime:      time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC),
			AgentStateFile: agentStateFile,
		}
		sessionData, _ := json.Marshal(session)
		sessionFilePath := filepath.Join(sessionsDir, "test-session-1.json")
		os.WriteFile(sessionFilePath, sessionData, 0644)
		defer os.Remove(sessionFilePath)

		// Redirect stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		mockFactory := func() (*runner.SessionManager, error) {
			return runner.NewSessionManagerWithDir(sessionsDir)
		}

		err := runHistoryCmd(mockFactory)
		assert.NoError(t, err)

		// Restore stdout and capture output
		w.Close()
		os.Stdout = oldStdout
		var buf bytes.Buffer
		buf.ReadFrom(r)
		output := buf.String()

		// Assertions: Check for key pieces of data, ignore whitespace issues
		assert.Contains(t, output, "test-session-1")
		assert.Contains(t, output, "completed")
		assert.Contains(t, output, "12345")      // Token count
		assert.Contains(t, output, "0.0123")     // Estimated cost
		assert.Contains(t, output, "2023-01-01") // Start time
	})

	// --- Test Case 3: Error Listing Sessions ---
	t.Run("ErrorListingSessions", func(t *testing.T) {
		unreadableDir := t.TempDir()
		// Make the directory unreadable to force an error in ListSessions
		assert.NoError(t, os.Chmod(unreadableDir, 0000))
		defer os.Chmod(unreadableDir, 0755) // Cleanup permissions

		mockFactory := func() (*runner.SessionManager, error) {
			return runner.NewSessionManagerWithDir(unreadableDir)
		}

		err := runHistoryCmd(mockFactory)
		assert.Error(t, err)
		assert.True(t, strings.Contains(err.Error(), "failed to list sessions"), "Error message should indicate a listing failure")
	})
}
