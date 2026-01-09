package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"recac/internal/runner"
	"recac/internal/ui"
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetStatus(t *testing.T) {
	// Setup: Create a temporary directory for sessions
	tempDir := t.TempDir()
	t.Setenv("HOME", tempDir) // Isolate session manager

	// We need to initialize the session manager to create the .recac/sessions directory
	_, err := runner.NewSessionManager()
	require.NoError(t, err, "failed to create session manager")

	// Create a fake session
	sessionName := fmt.Sprintf("test-session-%d", time.Now().UnixNano())
	fakeSession := &runner.SessionState{
		Name:      sessionName,
		PID:       os.Getpid(),
		StartTime: time.Now().Add(-10 * time.Minute),
		Status:    "running",
		LogFile:   "/tmp/test.log",
	}
	// Correctly construct the path where the session manager will look for the file
	sessionPath := filepath.Join(tempDir, ".recac", "sessions", fmt.Sprintf("%s.json", sessionName))
	require.NoError(t, os.MkdirAll(filepath.Dir(sessionPath), 0755))

	data, err := json.Marshal(fakeSession)
	require.NoError(t, err, "failed to marshal fake session")

	require.NoError(t, os.WriteFile(sessionPath, data, 0644), "failed to write fake session file")

	// Setup viper config
	viper.Set("provider", "test-provider")
	viper.Set("model", "test-model")
	viper.Set("config", "/tmp/config.yaml")
	defer viper.Reset()

	// Execute the function
	output := ui.GetStatus()

	// Assertions
	t.Run("Session Output", func(t *testing.T) {
		assert.Contains(t, output, "[Sessions]")
		assert.Contains(t, output, sessionName)
		assert.Contains(t, output, fmt.Sprintf("PID: %d", fakeSession.PID))
		assert.Contains(t, output, "Status: RUNNING") // The logic uppercases the status
	})

	t.Run("Docker Output", func(t *testing.T) {
		assert.Contains(t, output, "[Docker Environment]")
	})

	t.Run("Configuration Output", func(t *testing.T) {
		assert.Contains(t, output, "[Configuration]")
		assert.Contains(t, output, "Provider: test-provider")
		assert.Contains(t, output, "Model: test-model")
		assert.Contains(t, output, "Config File: /tmp/config.yaml")
	})
}

func TestGetStatus_NoSessions(t *testing.T) {
	// Setup: Create a temporary directory for sessions
	tempDir := t.TempDir()
	t.Setenv("HOME", tempDir) // Isolate session manager

	// Initialize session manager to ensure directories are created
	_, err := runner.NewSessionManager()
	require.NoError(t, err)

	// Execute the function with no sessions present
	output := ui.GetStatus()

	// Assertions
	assert.Contains(t, output, "No active or past sessions found.")
}
