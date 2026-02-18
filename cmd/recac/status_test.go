package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"recac/internal/agent"
	"recac/internal/runner"

	"regexp"

	"github.com/stretchr/testify/assert"
)

// setupStatusTest initializes a mock session manager and injects it.
func setupStatusTest(t *testing.T) (*MockSessionManager, func()) {
	t.Helper()
	mockSM := NewMockSessionManager()

	originalFactory := sessionManagerFactory
	sessionManagerFactory = func() (ISessionManager, error) {
		return mockSM, nil
	}

	return mockSM, func() {
		sessionManagerFactory = originalFactory
	}
}

// stripAnsi removes ANSI escape codes from a string
func stripAnsi(str string) string {
	ansi := regexp.MustCompile(`\x1b\[[0-9;]*m`)
	return ansi.ReplaceAllString(str, "")
}

// normalizeSpace replaces multiple spaces/tabs with a single space
func normalizeSpace(str string) string {
	space := regexp.MustCompile(`\s+`)
	return space.ReplaceAllString(str, " ")
}

func TestStatusCommand(t *testing.T) {
	t.Run("show status for a running session", func(t *testing.T) {
		mockSM, cleanup := setupStatusTest(t)
		defer cleanup()

		// --- Setup ---
		tempDir := t.TempDir()
		agentStateFile := filepath.Join(tempDir, ".agent_state.json")

		state := &agent.State{
			Model: "test-model",
			TokenUsage: agent.TokenUsage{
				TotalPromptTokens:   100,
				TotalResponseTokens: 200,
				TotalTokens:         300,
			},
			History: []agent.Message{
				{Role: "user", Content: "Initial goal", Timestamp: time.Now().Add(-10 * time.Minute)},
				{Role: "assistant", Content: "Okay, I will start working.", Timestamp: time.Now().Add(-5 * time.Minute)},
			},
		}
		stateBytes, _ := json.Marshal(state)
		os.WriteFile(agentStateFile, stateBytes, 0644)

		mockSM.Sessions["my-session"] = &runner.SessionState{
			Name:           "my-session",
			Status:         "running",
			Goal:           "Develop a new feature",
			StartTime:      time.Now().Add(-15 * time.Minute),
			AgentStateFile: agentStateFile,
		}

		// --- Execute ---
		rawOutput, err := executeCommand(rootCmd, "status", "my-session")
		output := normalizeSpace(stripAnsi(rawOutput))

		// --- Assert ---
		assert.NoError(t, err)
		assert.Contains(t, output, "Session: my-session")
		assert.Contains(t, output, "Goal: Develop a new feature")
		assert.Contains(t, output, "Status: running")
		assert.Contains(t, output, "Model: test-model")
		assert.Contains(t, output, "Tokens: 300 (Prompt: 100, Completion: 200)")
		assert.Contains(t, output, "Est. Cost:")
		assert.Contains(t, output, "Role: assistant")
		assert.Contains(t, output, "Content: Okay, I will start working.")
	})

	t.Run("show status for a completed session", func(t *testing.T) {
		mockSM, cleanup := setupStatusTest(t)
		defer cleanup()

		// --- Setup ---
		tempDir := t.TempDir()
		agentStateFile := filepath.Join(tempDir, ".agent_state.json")
		state := &agent.State{Model: "test-model-final"}
		stateBytes, _ := json.Marshal(state)
		os.WriteFile(agentStateFile, stateBytes, 0644)

		startTime := time.Now().Add(-30 * time.Minute)
		endTime := time.Now().Add(-5 * time.Minute)
		mockSM.Sessions["completed-session"] = &runner.SessionState{
			Name:           "completed-session",
			Status:         "completed",
			StartTime:      startTime,
			EndTime:        endTime,
			AgentStateFile: agentStateFile,
		}

		// --- Execute ---
		rawOutput, err := executeCommand(rootCmd, "status", "completed-session")
		output := normalizeSpace(stripAnsi(rawOutput))

		// --- Assert ---
		assert.NoError(t, err)
		assert.Contains(t, output, "Session: completed-session")
		assert.Contains(t, output, "Status: completed")
		assert.True(t, strings.Contains(output, "25m0s")) // Duration
	})

	t.Run("gracefully handles missing agent state file", func(t *testing.T) {
		mockSM, cleanup := setupStatusTest(t)
		defer cleanup()

		// --- Setup ---
		mockSM.Sessions["no-state-session"] = &runner.SessionState{
			Name:           "no-state-session",
			Status:         "running",
			AgentStateFile: "/path/to/non/existent/file.json",
		}

		// --- Execute ---
		output, err := executeCommand(rootCmd, "status", "no-state-session")
		// Not stripping ANSI here because the error path might be plain text,
		// but checking both is safer.
		cleanOutput := stripAnsi(output)

		// --- Assert ---
		assert.NoError(t, err)
		assert.Contains(t, cleanOutput, "Session 'no-state-session' found, but agent state is not available.")
		assert.Contains(t, cleanOutput, "Status: running")
	})

	t.Run("defaults to most recent session when no name is provided", func(t *testing.T) {
		mockSM, cleanup := setupStatusTest(t)
		defer cleanup()

		// --- Setup ---
		tempDir := t.TempDir()
		agentStateFile := filepath.Join(tempDir, ".agent_state.json")
		state := &agent.State{Model: "recent-model"}
		stateBytes, _ := json.Marshal(state)
		os.WriteFile(agentStateFile, stateBytes, 0644)

		mockSM.Sessions["older-session"] = &runner.SessionState{
			Name:      "older-session",
			StartTime: time.Now().Add(-1 * time.Hour),
		}
		mockSM.Sessions["recent-session"] = &runner.SessionState{
			Name:           "recent-session",
			StartTime:      time.Now().Add(-10 * time.Minute),
			AgentStateFile: agentStateFile,
		}

		// --- Execute ---
		rawOutput, err := executeCommand(rootCmd, "status")
		output := normalizeSpace(stripAnsi(rawOutput))

		// --- Assert ---
		assert.NoError(t, err)
		assert.Contains(t, output, "showing status for most recent session: recent-session")
		assert.Contains(t, output, "Session: recent-session")
		assert.Contains(t, output, "Model: recent-model")
	})

	t.Run("reports error for non-existent session", func(t *testing.T) {
		_, cleanup := setupStatusTest(t)
		defer cleanup()

		// --- Execute ---
		_, err := executeCommand(rootCmd, "status", "non-existent-session")

		// --- Assert ---
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "could not load session 'non-existent-session'")
	})
}
