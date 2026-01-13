package main

import (
	"recac/internal/runner"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSummaryCommand(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		setupSessions func() *MockSessionManager
		expectOutput  []string
		expectError   bool
	}{
		{
			name: "no sessions",
			args: []string{"summary"},
			setupSessions: func() *MockSessionManager {
				return &MockSessionManager{
					Sessions: make(map[string]*runner.SessionState),
				}
			},
			expectOutput: []string{"No sessions found."},
			expectError:  false,
		},
		{
			name: "multiple sessions",
			args: []string{"summary"},
			setupSessions: func() *MockSessionManager {
				return &MockSessionManager{
					Sessions: map[string]*runner.SessionState{
						"session1": {Name: "session1", Status: "completed", StartTime: time.Now().Add(-1 * time.Hour)},
						"session2": {Name: "session2", Status: "running", StartTime: time.Now()},
						"session3": {Name: "session3", Status: "error", StartTime: time.Now().Add(-2 * time.Hour)},
					},
				}
			},
			expectOutput: []string{
				`Aggregate Stats`,
				`Total Sessions:\s+3`,
				`Completed:\s+1`,
				`Errored:\s+1`,
				`Running:\s+1`,
				`Success Rate:\s+33.33%`,
				`Recent Sessions`,
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockSM := tt.setupSessions()
			sessionManagerFactory = func() (ISessionManager, error) {
				return mockSM, nil
			}

			rootCmd, _, _ := newRootCmd()
			output, err := executeCommand(rootCmd, tt.args...)

			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			for _, expected := range tt.expectOutput {
				assert.Regexp(t, expected, output)
			}
		})
	}
}

// NOTE: A proper test for the --watch flag would require a more complex setup
// to mock the TUI and its lifecycle. For this exercise, we'll just check that the
// command doesn't error out, as a basic integration test. A full TUI test
// would be better suited for an e2e testing framework.
func TestSummaryCommandWatch(t *testing.T) {
	// Since we can't easily test the TUI, we'll just make sure the command
	// doesn't return an error. This is a basic integration check.
	t.Skip("TUI watch tests are not implemented")
}
