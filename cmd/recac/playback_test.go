package main

import (
	"os"
	"path/filepath"
	"testing"

	"recac/internal/runner"
	"recac/internal/ui"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

func TestPlaybackCommand_Run(t *testing.T) {
	// Save original factories and vars
	originalFactory := sessionManagerFactory
	originalRunTUI := runPlaybackTUI
	defer func() {
		sessionManagerFactory = originalFactory
		runPlaybackTUI = originalRunTUI
	}()

	// Mock Session Manager
	mockSM := NewMockSessionManager()
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "test.log")
	require.NoError(t, os.WriteFile(logPath, []byte(`{"msg":"test log"}`), 0644))

	mockSM.Sessions["test-session"] = &runner.SessionState{
		Name:    "test-session",
		LogFile: logPath,
	}

	sessionManagerFactory = func() (ISessionManager, error) {
		return mockSM, nil
	}

	// Mock TUI Runner
	tuiCalled := false
	runPlaybackTUI = func(m tea.Model) error {
		tuiCalled = true
		// Verify model is initialized with correct data
		playbackModel, ok := m.(ui.PlaybackModel)
		assert.True(t, ok, "Expected ui.PlaybackModel")
		// We can't easily inspect internal fields of PlaybackModel without exporting them or using reflection,
		// but we can check the View output or similar if needed. For now, just type check is good.
		_ = playbackModel
		return nil
	}

	// Run Command
	// We call RunE directly to avoid side effects of cobra execution in this unit test
	err := playbackCmd.RunE(playbackCmd, []string{"test-session"})
	require.NoError(t, err)
	assert.True(t, tuiCalled, "TUI runner should be called")
}
