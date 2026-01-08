package main

import (
	"bytes"
	"fmt"
	"recac/internal/runner"
	"strings"
	"testing"
	"time"
)

// MockSessionManager for testing
type MockSessionManager struct {
	Sessions    []*runner.SessionState
	StopCalled  bool
	StoppedName string
}

func (m *MockSessionManager) ListSessions() ([]*runner.SessionState, error) {
	return m.Sessions, nil
}

func (m *MockSessionManager) StopSession(name string) error {
	m.StopCalled = true
	m.StoppedName = name
	for _, s := range m.Sessions {
		if s.Name == name {
			s.Status = "stopped"
			return nil
		}
	}
	return fmt.Errorf("session not found: %s", name)
}

func (m *MockSessionManager) IsProcessRunning(pid int) bool {
	return true // Assume all processes are running for test purposes
}

func (m *MockSessionManager) LoadSession(name string) (*runner.SessionState, error) {
	for _, s := range m.Sessions {
		if s.Name == name {
			return s, nil
		}
	}
	return nil, fmt.Errorf("session not found: %s", name)
}

func (m *MockSessionManager) StartSession(name string, command []string, workspace string) (*runner.SessionState, error) {
	newState := &runner.SessionState{Name: name, Command: command, Workspace: workspace, Status: "running"}
	m.Sessions = append(m.Sessions, newState)
	return newState, nil
}

func (m *MockSessionManager) GetSessionPath(name string) string {
	return "" // Not needed for this test
}

func TestStopInteractive(t *testing.T) {
	// 1. Setup Mock Session Manager
	mockSM := &MockSessionManager{
		Sessions: []*runner.SessionState{
			{Name: "session-1", PID: 123, Status: "running", StartTime: time.Now()},
			{Name: "session-2", PID: 456, Status: "completed", StartTime: time.Now()},
			{Name: "session-3", PID: 789, Status: "running", StartTime: time.Now()},
		},
	}

	// 2. Override the factory to return our mock
	originalNewSessionManager := newSessionManager
	newSessionManager = func() (ISessionManager, error) {
		return mockSM, nil
	}
	defer func() { newSessionManager = originalNewSessionManager }()

	// 3. Simulate user input by creating a pipe
	input := "2\n" // User chooses the second running session (session-3)
	in := bytes.NewBufferString(input)
	rootCmd.SetIn(in)

	// 4. Execute the command using the shared helper
	output, err := executeCommand(rootCmd, "stop")

	// 5. Assertions
	if err != nil {
		t.Fatalf("executeCommand failed: %v", err)
	}

	expectedPrompts := []string{
		"Running sessions:",
		"1: session-1",
		"2: session-3",
		"Enter the number of the session to stop:",
	}

	for _, p := range expectedPrompts {
		if !strings.Contains(output, p) {
			t.Errorf("Expected output to contain '%s', but it did not.\nGot: %s", p, output)
		}
	}

	if !mockSM.StopCalled {
		t.Error("Expected StopSession to be called, but it was not.")
	}

	if mockSM.StoppedName != "session-3" {
		t.Errorf("Expected session 'session-3' to be stopped, but got '%s'", mockSM.StoppedName)
	}

	if !strings.Contains(output, "Session 'session-3' stopped successfully") {
		t.Errorf("Expected success message for session-3, but it was not found.\nGot: %s", output)
	}
}
