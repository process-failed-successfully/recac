package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"recac/internal/runner"
	"strings"
	"testing"
	"time"
)

func TestHistoryCommand(t *testing.T) {
	// 1. Setup: Create a temporary directory for session files
	tmpDir, err := os.MkdirTemp("", "recac-history-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Override the factory function to use a test-specific session manager
	originalNewSessionManager := newSessionManager
	newSessionManager = func() (ISessionManager, error) {
		return runner.NewSessionManagerWithDir(tmpDir)
	}
	defer func() { newSessionManager = originalNewSessionManager }()

	// 2. Create mock session files
	now := time.Now()
	sessions := []*runner.SessionState{
		{
			Name:      "running-session",
			PID:       os.Getpid(), // Use a real PID to avoid being marked as 'completed'
			StartTime: now.Add(-10 * time.Minute),
			Status:    "running",
			Workspace: "/tmp/ws1",
		},
		{
			Name:      "completed-session",
			PID:       12345, // A fake, non-running PID
			StartTime: now.Add(-1 * time.Hour),
			EndTime:   now.Add(-30 * time.Minute),
			Status:    "completed",
			Workspace: "/tmp/ws2",
		},
		{
			Name:      "stopped-session",
			PID:       54321, // A fake, non-running PID
			StartTime: now.Add(-2 * time.Hour),
			EndTime:   now.Add(-1 * time.Hour),
			Status:    "stopped",
			Workspace: "/tmp/ws3",
		},
	}

	for _, s := range sessions {
		data, _ := json.Marshal(s)
		path := filepath.Join(tmpDir, s.Name+".json")
		if err := os.WriteFile(path, data, 0644); err != nil {
			t.Fatalf("Failed to write mock session file: %v", err)
		}
	}

	// 3. Execute the command using the test helper and capture output
	// We pass the rootCmd to the helper, which sets up buffers and executes.
	result, err := executeCommand(rootCmd, "history")
	if err != nil {
		t.Fatalf("executeCommand failed: %v", err)
	}

	// 4. Assert the output
	// Check for headers
	if !strings.Contains(result, "NAME") || !strings.Contains(result, "STATUS") || !strings.Contains(result, "DURATION") {
		t.Errorf("Output is missing expected headers. Got:\n%s", result)
	}

	// Check for session names (order is now predictable due to sorting)
	lines := strings.Split(strings.TrimSpace(result), "\n")
	if len(lines) < 5 { // 2 header lines + 3 data lines
		t.Fatalf("Expected at least 5 lines of output, got %d. Output:\n%s", len(lines), result)
	}

	// The order should be stopped, completed, running based on StartTime
	if !strings.Contains(lines[2], "stopped-session") {
		t.Errorf("Expected first session to be 'stopped-session', but got: %s", lines[2])
	}
	if !strings.Contains(lines[3], "completed-session") {
		t.Errorf("Expected second session to be 'completed-session', but got: %s", lines[3])
	}
	if !strings.Contains(lines[4], "running-session") {
		t.Errorf("Expected third session to be 'running-session', but got: %s", lines[4])
	}

	// Check for a calculated duration.
	if !strings.Contains(result, "1h0m0s") { // Duration of stopped-session
		t.Errorf("Output is missing expected duration '1h0m0s'. Got:\n%s", result)
	}
	if !strings.Contains(result, "30m0s") { // Duration of completed-session
		t.Errorf("Output is missing expected duration '30m0s'. Got:\n%s", result)
	}
}
