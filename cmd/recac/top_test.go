package main

import (
	"recac/internal/ui"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTopCommand(t *testing.T) {
	// --- Setup ---
	// Mock the session manager to avoid filesystem dependency
	oldSessionManager := sessionManagerFactory
	sessionManagerFactory = func() (ISessionManager, error) {
		return &MockSessionManager{}, nil
	}
	defer func() { sessionManagerFactory = oldSessionManager }()

	// Mock the TUI starter
	var topDashboardStarted bool
	originalStartTopDashboard := ui.StartTopDashboard
	ui.StartTopDashboard = func() error {
		topDashboardStarted = true
		return nil
	}
	defer func() { ui.StartTopDashboard = originalStartTopDashboard }()

	// --- Execution ---
	_, err := executeCommand(rootCmd, "top")

	// --- Assertions ---
	assert.NoError(t, err, "Expected no error from top command")
	assert.True(t, topDashboardStarted, "Expected StartTopDashboard to be called")
}
