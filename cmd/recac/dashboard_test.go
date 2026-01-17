package main

import (
	"recac/internal/agent"
	"recac/internal/runner"
	"recac/internal/ui"
	"testing"
	"time"

	"github.com/spf13/cobra"
)

func TestDashboardCmd(t *testing.T) {
	// 1. Setup Mock Session Manager
	mockSM := NewMockSessionManager()
	sessionManagerFactory = func() (ISessionManager, error) {
		return mockSM, nil
	}

	// 2. Setup Mock UI Starter to avoid actual TUI
	originalStarter := ui.StartSessionDashboard
	defer func() { ui.StartSessionDashboard = originalStarter }()

	var capturedSessionName string
	// Override the variable in the UI package
	ui.StartSessionDashboard = func(name string) error {
		capturedSessionName = name
		return nil
	}

	// 3. Create a Dummy Session
	mockSM.Sessions["test-session"] = &runner.SessionState{
		Name:      "test-session",
		Status:    "running",
		StartTime: time.Now(),
	}

	// 4. Test Case: Explicit Session Name
	t.Run("Explicit Session Name", func(t *testing.T) {
		capturedSessionName = "" // Reset

		err := dashboardCmd.RunE(dashboardCmd, []string{"test-session"})
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		if capturedSessionName != "test-session" {
			t.Errorf("Expected session name 'test-session', got '%s'", capturedSessionName)
		}
	})

	// 5. Test Case: Auto-detect Recent Session
	t.Run("Auto Detect Recent", func(t *testing.T) {
		capturedSessionName = "" // Reset
		mockSM.Sessions["old-session"] = &runner.SessionState{
			Name:      "old-session",
			Status:    "completed",
			StartTime: time.Now().Add(-1 * time.Hour),
		}
		mockSM.Sessions["new-session"] = &runner.SessionState{
			Name:      "new-session",
			Status:    "running",
			StartTime: time.Now(),
		}

		err := dashboardCmd.RunE(dashboardCmd, []string{})
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		if capturedSessionName != "new-session" {
			t.Errorf("Expected session name 'new-session', got '%s'", capturedSessionName)
		}
	})

	// 6. Test Data Injection Logic (Sanity Check)
	t.Run("Data Injection", func(t *testing.T) {
		// Run the command to trigger the injection setup
		_ = dashboardCmd.RunE(dashboardCmd, []string{"test-session"})

		// Verify GetSessionDetail
		if ui.GetSessionDetail == nil {
			t.Fatal("GetSessionDetail was not injected")
		}

		s, err := ui.GetSessionDetail("test-session")
		if err != nil {
			t.Fatalf("GetSessionDetail failed: %v", err)
		}
		if s.Name != "test-session" {
			t.Errorf("Expected mapped session name")
		}

		// Verify GetAgentState (Mock loadAgentState)
		originalLoad := loadAgentState
		defer func() { loadAgentState = originalLoad }()
		loadAgentState = func(file string) (*agent.State, error) {
			return &agent.State{Model: "gpt-mock"}, nil
		}

		state, err := ui.GetAgentState("test-session")
		if err != nil {
			t.Fatalf("GetAgentState failed: %v", err)
		}
		if state.Model != "gpt-mock" {
			t.Errorf("Expected state model match")
		}
	})
}

// Helper to avoid polluting global rootCmd
func newRootCmdForTest() *cobra.Command {
	return &cobra.Command{Use: "recac"}
}
