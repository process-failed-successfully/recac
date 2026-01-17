package main

import (
	"bytes"
	"fmt"
	"os"
	"recac/internal/runner"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLogsCmd(t *testing.T) {
	// Create a temporary directory for log files
	tmpDir, err := os.MkdirTemp("", "recac-logs-test-")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create a mock log file
	logFile := fmt.Sprintf("%s/test.log", tmpDir)
	err = os.WriteFile(logFile, []byte("line 1\nline 2\nfiltered line\n"), 0644)
	require.NoError(t, err)

	// Setup Mock SessionManager
	mockSM := NewMockSessionManager()
	mockSM.Sessions["test-session"] = &runner.SessionState{
		Name:      "test-session",
		Status:    "running",
		LogFile:   logFile,
		StartTime: time.Now(),
	}

	// Override factory
	originalFactory := sessionManagerFactory
	sessionManagerFactory = func() (ISessionManager, error) {
		return mockSM, nil
	}
	defer func() { sessionManagerFactory = originalFactory }()

	// Override exit to prevent test termination
	originalExit := exit
	var exitCode int
	exit = func(code int) {
		exitCode = code
		panic(fmt.Sprintf("exit-%d", code))
	}
	defer func() { exit = originalExit }()

	t.Run("Get Logs Success", func(t *testing.T) {
		cmd := NewLogsCmd()
		b := new(bytes.Buffer)
		cmd.SetOut(b)
		cmd.SetErr(b)
		cmd.SetArgs([]string{"test-session"})

		err := cmd.Execute()
		assert.NoError(t, err)
		assert.Contains(t, b.String(), "line 1")
		assert.Contains(t, b.String(), "line 2")
	})

	t.Run("Get Logs Filtered", func(t *testing.T) {
		cmd := NewLogsCmd()
		b := new(bytes.Buffer)
		cmd.SetOut(b)
		cmd.SetErr(b)
		cmd.SetArgs([]string{"test-session", "--filter", "filtered"})

		err := cmd.Execute()
		assert.NoError(t, err)
		assert.Contains(t, b.String(), "filtered line")
		assert.NotContains(t, b.String(), "line 1")
	})

	t.Run("Get Logs No Session", func(t *testing.T) {
		cmd := NewLogsCmd()
		b := new(bytes.Buffer)
		cmd.SetOut(b)
		cmd.SetErr(b)
		cmd.SetArgs([]string{"non-existent"})

		defer func() {
			if r := recover(); r != nil {
				assert.Equal(t, "exit-1", r)
				assert.Equal(t, 1, exitCode)
			}
		}()

		cmd.Execute()
	})

	t.Run("Logs All Sessions", func(t *testing.T) {
		cmd := NewLogsCmd()
		b := new(bytes.Buffer)
		cmd.SetOut(b)
		cmd.SetErr(b)
		cmd.SetArgs([]string{"--all"})

		err := cmd.Execute()
		assert.NoError(t, err)
		assert.Contains(t, b.String(), "[test-session] line 1")
	})
}
