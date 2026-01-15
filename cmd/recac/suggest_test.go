package main

import (
	"recac/internal/runner"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSuggestCmd(t *testing.T) {
	// Keep a reference to the original factory
	originalFactory := sessionManagerFactory
	defer func() {
		// Restore the original factory after all tests in this function have run
		sessionManagerFactory = originalFactory
	}()

	testCases := []struct {
		name             string
		sessions         map[string]*runner.SessionState
		expectedContains []string
	}{
		{
			name:     "No Sessions",
			sessions: map[string]*runner.SessionState{},
			expectedContains: []string{
				"No sessions found",
				"recac start",
			},
		},
		{
			name: "Running Session",
			sessions: map[string]*runner.SessionState{
				"running-session": {Name: "running-session", Status: "running", StartTime: time.Now()},
			},
			expectedContains: []string{
				"Session 'running-session' is running",
				"recac logs -f running-session",
				"recac status running-session",
			},
		},
		{
			name: "Completed Session",
			sessions: map[string]*runner.SessionState{
				"completed-session": {Name: "completed-session", Status: "completed", StartTime: time.Now()},
			},
			expectedContains: []string{
				"Session 'completed-session' completed successfully",
				"recac history completed-session",
				"recac archive completed-session",
				"recac start",
			},
		},
		{
			name: "Error Session",
			sessions: map[string]*runner.SessionState{
				"error-session": {Name: "error-session", Status: "error", StartTime: time.Now()},
			},
			expectedContains: []string{
				"Session 'error-session' failed",
				"recac logs error-session",
				"recac history error-session",
			},
		},
		{
			name: "Unknown Status",
			sessions: map[string]*runner.SessionState{
				"unknown-session": {Name: "unknown-session", Status: "unknown", StartTime: time.Now()},
			},
			expectedContains: []string{
				"Latest session 'unknown-session' has status 'unknown'",
			},
		},
		{
			name: "Multiple Sessions - Latest is Running",
			sessions: map[string]*runner.SessionState{
				"older-session":  {Name: "older-session", Status: "completed", StartTime: time.Now().Add(-1 * time.Hour)},
				"latest-session": {Name: "latest-session", Status: "running", StartTime: time.Now()},
			},
			expectedContains: []string{
				"Session 'latest-session' is running",
				"recac logs -f latest-session",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a new mock for each test case
			mockSm := NewMockSessionManager()
			mockSm.Sessions = tc.sessions

			// Override the factory to return our mock
			sessionManagerFactory = func() (ISessionManager, error) {
				return mockSm, nil
			}

			// Execute the command and capture the output
			output, err := executeCommand(rootCmd, "suggest")

			// Assertions
			assert.NoError(t, err)
			for _, expected := range tc.expectedContains {
				assert.True(t, strings.Contains(output, expected), "Expected output to contain '%s', but got '%s'", expected, output)
			}
		})
	}
}
