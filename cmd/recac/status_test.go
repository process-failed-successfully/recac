package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"recac/internal/runner"
	"strings"
	"testing"
	"time"

	"github.com/spf13/viper"
)

// captureOutput executes a function and captures its standard output.
func captureOutput(f func() error) (string, error) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := f()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String(), err
}

func TestShowStatus(t *testing.T) {
	// Setup: Create a temporary session manager and a fake session file
	sm, err := runner.NewSessionManager()
	if err != nil {
		t.Fatalf("failed to create session manager: %v", err)
	}

	sessionName := fmt.Sprintf("test-session-%d", time.Now().UnixNano())
	fakeSession := &runner.SessionState{
		Name:      sessionName,
		PID:       os.Getpid(), // Use the current process ID, which is guaranteed to be running
		StartTime: time.Now().Add(-10 * time.Minute),
		Status:    "running",
		LogFile:   "/tmp/test.log",
	}
	sessionPath := sm.GetSessionPath(sessionName)

	data, err := json.Marshal(fakeSession)
	if err != nil {
		t.Fatalf("failed to marshal fake session: %v", err)
	}

	if err := os.WriteFile(sessionPath, data, 0644); err != nil {
		t.Fatalf("failed to write fake session file: %v", err)
	}
	defer os.Remove(sessionPath) // Cleanup

	// Setup viper config
	viper.Set("provider", "test-provider")
	viper.Set("model", "test-model")
	viper.Set("config", "/tmp/config.yaml")
	defer viper.Reset() // Cleanup viper

	// Execute the command and capture output
	output, err := captureOutput(showStatus)
	if err != nil {
		t.Errorf("showStatus() returned an error: %v", err)
	}

	// Assertions
	t.Run("Session Output", func(t *testing.T) {
		if !strings.Contains(output, "[Sessions]") {
			t.Error("output should contain '[Sessions]'")
		}
		if !strings.Contains(output, sessionName) {
			t.Errorf("output should contain session name '%s'", sessionName)
		}
		if !strings.Contains(output, fmt.Sprintf("PID: %d", fakeSession.PID)) {
			t.Errorf("output should contain 'PID: %d'", fakeSession.PID)
		}
		if !strings.Contains(output, "Status: RUNNING") {
			t.Error("output should contain 'Status: RUNNING'")
		}
	})

	t.Run("Docker Output", func(t *testing.T) {
		if !strings.Contains(output, "[Docker Environment]") {
			t.Error("output should contain '[Docker Environment]'")
		}
	})

	t.Run("Configuration Output", func(t *testing.T) {
		if !strings.Contains(output, "[Configuration]") {
			t.Error("output should contain '[Configuration]'")
		}
		if !strings.Contains(output, "Provider: test-provider") {
			t.Error("output should contain 'Provider: test-provider'")
		}
		if !strings.Contains(output, "Model: test-model") {
			t.Error("output should contain 'Model: test-model'")
		}
		if !strings.Contains(output, "Config File: /tmp/config.yaml") {
			t.Error("output should contain 'Config File: /tmp/config.yaml'")
		}
	})
}

func TestShowStatus_NoSessions(t *testing.T) {
	// Execute the command with no sessions present
	output, err := captureOutput(showStatus)
	if err != nil {
		t.Errorf("showStatus() returned an error: %v", err)
	}

	// Assertions
	if !strings.Contains(output, "No active or past sessions found.") {
		t.Error("output should contain 'No active or past sessions found.'")
	}
}
