package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"recac/internal/runner"
	"recac/internal/ui"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseLogLines(t *testing.T) {
	// 1. Create temporary log file content
	jsonl := `{"time":"2023-10-27T10:00:00Z","level":"INFO","msg":"Starting session","session":"test-1"}
{"time":"2023-10-27T10:00:01Z","level":"DEBUG","msg":"Thinking","prompt":"User asked..."}
{"time":"2023-10-27T10:00:02Z","level":"ERROR","msg":"Failed","error":"connection refused"}
Non-JSON line here
`

	// 2. Parse
	entries, err := ui.ParseLogLines([]byte(jsonl))
	require.NoError(t, err)

	// 3. Assertions
	require.Len(t, entries, 4)

	// Entry 0
	assert.Equal(t, "INFO", entries[0].Level)
	assert.Equal(t, "Starting session", entries[0].Msg)
	assert.Equal(t, "test-1", entries[0].Raw["session"])
	expectedTime, _ := time.Parse(time.RFC3339, "2023-10-27T10:00:00Z")
	assert.Equal(t, expectedTime, entries[0].Time)

	// Entry 1
	assert.Equal(t, "DEBUG", entries[1].Level)
	assert.Equal(t, "Thinking", entries[1].Msg)
	assert.Equal(t, "User asked...", entries[1].Raw["prompt"])

	// Entry 2
	assert.Equal(t, "ERROR", entries[2].Level)
	assert.Equal(t, "Failed", entries[2].Msg)

	// Entry 3 (Non-JSON)
	assert.Equal(t, "TEXT", entries[3].Level)
	assert.Equal(t, "Non-JSON line here", entries[3].Msg)
}

func TestPlaybackModelInit(t *testing.T) {
	entries := []ui.LogEntry{
		{Level: "INFO", Msg: "Test_1"},
		{Level: "ERROR", Msg: "Test_2"},
	}

	model := ui.NewPlaybackModel(entries)

	// Send WindowSizeMsg to ensure list renders items
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 80, Height: 20})

	view := model.View()
	// Check for title or content
	assert.Contains(t, view, "Session Playback")
	assert.Contains(t, view, "Test_1")
	assert.Contains(t, view, "Test_2")
}

// Helper to cast model update
func updateModel(m tea.Model, msg tea.Msg) (ui.PlaybackModel, tea.Cmd) {
	updated, cmd := m.Update(msg)
	return updated.(ui.PlaybackModel), cmd
}

func TestPlaybackCommand_Setup(t *testing.T) {
	// This tests the setup logic using the existing MockSessionManager

	// Save original factory
	originalFactory := sessionManagerFactory
	defer func() { sessionManagerFactory = originalFactory }()

	// Create temp session logs
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "test-session.log")
	err := os.WriteFile(logPath, []byte(`{"level":"INFO","msg":"Integration Test"}`), 0644)
	require.NoError(t, err)

	// Configure Mock
	mockSM := NewMockSessionManager()
	mockSM.Sessions["test-session"] = &runner.SessionState{
		Name:    "test-session",
		LogFile: logPath,
	}

	// Override Factory
	sessionManagerFactory = func() (ISessionManager, error) {
		return mockSM, nil
	}

	// Verify we can get the logs via the factory (simulating the command logic)
	sm, err := sessionManagerFactory()
	require.NoError(t, err)

	retrievedPath, err := sm.GetSessionLogs("test-session")
	require.NoError(t, err)
	assert.Equal(t, logPath, retrievedPath)
}
