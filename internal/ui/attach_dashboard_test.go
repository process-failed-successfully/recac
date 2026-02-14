package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"recac/internal/runner"
	"strings"
	"testing"
	"time"
)

func TestAttachDashboard_Logs(t *testing.T) {
	// Setup temp log file
	tmpDir, err := os.MkdirTemp("", "recac-test-attach")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	logPath := filepath.Join(tmpDir, "session.log")
	initialContent := "line 1\nline 2\n"
	if err := os.WriteFile(logPath, []byte(initialContent), 0600); err != nil {
		t.Fatal(err)
	}

	// Mock GetSession
	originalGetSession := GetSession
	defer func() { GetSession = originalGetSession }()

	sessionName := "test-session"
	mockSession := &runner.SessionState{
		Name:    sessionName,
		Status:  "running",
		LogFile: logPath,
		PID:     12345,
		Goal:    "Test Goal",
	}

	GetSession = func(name string) (*runner.SessionState, error) {
		if name == sessionName {
			return mockSession, nil
		}
		return nil, fmt.Errorf("session not found")
	}

	// Initialize model
	model := NewAttachDashboardModel(sessionName)
	model.ready = true
	model.width = 80
	model.height = 24
	model.session = mockSession // Simulate loaded session

	// Test Initial Read (Offset 0)
	// We need to simulate the tea.Cmd execution manually
	readCmd := readLogsCmd(sessionName, 0)
	msg := readCmd()

	logMsg, ok := msg.(logReadMsg)
	if !ok {
		t.Fatalf("Expected logReadMsg, got %T", msg)
	}
	if logMsg.err != nil {
		t.Fatalf("Unexpected error: %v", logMsg.err)
	}
	if logMsg.content != initialContent {
		t.Errorf("Expected content %q, got %q", initialContent, logMsg.content)
	}
	if logMsg.newOffset != int64(len(initialContent)) {
		t.Errorf("Expected offset %d, got %d", len(initialContent), logMsg.newOffset)
	}

	// Update model with logs
	m, _ := model.Update(logMsg)
	model = m.(attachDashboardModel)

	// Check log content directly
	if !strings.Contains(model.logContent, "line 1") {
		t.Errorf("Log content missing line 1: %s", model.logContent)
	}

	// Append to log file
	newContent := "line 3\n"
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(newContent); err != nil {
		t.Fatal(err)
	}
	f.Close()

	// Test Subsequent Read from new offset
	readCmd = readLogsCmd(sessionName, logMsg.newOffset)
	msg = readCmd()

	logMsg, ok = msg.(logReadMsg)
	if !ok {
		t.Fatalf("Expected logReadMsg, got %T", msg)
	}
	if logMsg.err != nil {
		t.Fatalf("Unexpected error: %v", logMsg.err)
	}
	if logMsg.content != newContent {
		t.Errorf("Expected content %q, got %q", newContent, logMsg.content)
	}

	// Update model again
	m, _ = model.Update(logMsg)
	model = m.(attachDashboardModel)
	if !strings.Contains(model.logContent, "line 3") {
		t.Errorf("Log content missing line 3: %s", model.logContent)
	}
}

func TestAttachDashboard_Tick(t *testing.T) {
	// Verify that tick updates session state and triggers log read

	// Mock GetSession
	originalGetSession := GetSession
	defer func() { GetSession = originalGetSession }()

	sessionName := "tick-session"
	logPath := "dummy.log" // Not used for this test part really
	mockSession := &runner.SessionState{
		Name:    sessionName,
		Status:  "running",
		LogFile: logPath,
	}

	callCount := 0
	GetSession = func(name string) (*runner.SessionState, error) {
		callCount++
		if name == sessionName {
			if callCount > 1 {
				mockSession.Status = "completed"
			}
			return mockSession, nil
		}
		return nil, fmt.Errorf("session not found")
	}

	model := NewAttachDashboardModel(sessionName)
	model.ready = true

	// Simulate tick
	tickMsg := attachTickMsg(time.Now())
	m, cmd := model.Update(tickMsg)
	model = m.(attachDashboardModel)

	// Check if session status updated
	if model.session == nil {
		t.Fatal("Session should not be nil after tick")
	}
	if model.session.Status != "running" {
		t.Errorf("Expected status running, got %s", model.session.Status)
	}

	// Simulate another tick
	m, cmd = model.Update(tickMsg)
	model = m.(attachDashboardModel)

	if model.session.Status != "completed" {
		t.Errorf("Expected status completed, got %s", model.session.Status)
	}

	// Verify returned command is not nil (it should batch tick and readLogs)
	if cmd == nil {
		t.Error("Expected cmd from tick update")
	}
}

func TestAttachDashboard_LogTruncation(t *testing.T) {
	// Create a model with almost 1MB of logs
	sessionName := "truncate-session"
	model := NewAttachDashboardModel(sessionName)
	model.ready = true

	// Fill buffer to max - 10 bytes
	// Use 'x' to distinguish
	padding := strings.Repeat("x", maxLogSize - 10)
	model.logContent = padding

	// Add 20 bytes
	newContent := strings.Repeat("y", 20)
	msg := logReadMsg{
		content: newContent,
		newOffset: 12345, // Irrelevant
	}

	m, _ := model.Update(msg)
	model = m.(attachDashboardModel)

	if len(model.logContent) > maxLogSize {
		t.Errorf("Log content exceeds max size: %d > %d", len(model.logContent), maxLogSize)
	}

	// It should contain the new content at the end
	if !strings.HasSuffix(model.logContent, newContent) {
		t.Error("Log content missing new content at end")
	}

	if len(model.logContent) != maxLogSize {
		t.Errorf("Expected length %d, got %d", maxLogSize, len(model.logContent))
	}

	// Verify first char is 'y' or 'x' depending on cut
	// We cut 10 bytes. The first 10 bytes of 'y' + 'x' padding logic
	// Original: 1MB-10 'x'. Added 20 'y'. Total 1MB+10.
	// Cut: 10 bytes from start.
	// Result: (1MB-20) 'x' + 20 'y'.
	// Wait, len is maxLogSize.
	// 1048576 bytes.

	// Check if truncated from start
	if !strings.HasPrefix(model.logContent, "xxxxxxxxxx") {
		// Just checking it still has padding
		t.Error("Log content should start with padding")
	}
}
