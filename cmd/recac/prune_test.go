package main

import (
	"os"
	"path/filepath"
	"recac/internal/runner"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPruneCommand(t *testing.T) {
	// Use the real session manager pointed at a temporary directory.
	sm, cleanup := setupTestSessionManager(t)
	defer cleanup()

	now := time.Now()
	sessions := []*runner.SessionState{
		// This session will be treated as "running" because its PID is the current process's PID.
		{Name: "running-recent", Status: "running", StartTime: now.Add(-1 * time.Hour), PID: os.Getpid()},
		{Name: "running-old", Status: "running", StartTime: now.Add(-48 * time.Hour), PID: os.Getpid()},
		{Name: "completed-recent", Status: "completed", StartTime: now.Add(-2 * time.Hour), PID: 0},
		{Name: "error-old", Status: "error", StartTime: now.Add(-48 * time.Hour), PID: 0},
		{Name: "completed-old", Status: "completed", StartTime: now.Add(-72 * time.Hour), PID: 0},
	}

	for _, s := range sessions {
		// Create dummy log files that the prune command can delete.
		logPath := filepath.Join(sm.SessionsDir(), s.Name+".log")
		s.LogFile = logPath
		require.NoError(t, os.WriteFile(logPath, []byte("log"), 0644))
		require.NoError(t, sm.SaveSession(s))
	}

	testCases := []struct {
		name            string
		args            []string
		expectedToPrune []string
		expectedToKeep  []string
		expectedOutput  string
		expectError     bool
	}{
		{
			name:            "default prune (non-running)",
			args:            []string{"prune"},
			expectedToPrune: []string{"completed-recent", "error-old", "completed-old"},
			expectedToKeep:  []string{"running-recent", "running-old"},
			expectedOutput:  "Pruned 3 session(s).",
		},
		{
			name:            "prune --all",
			args:            []string{"prune", "--all"},
			expectedToPrune: []string{"running-recent", "running-old", "completed-recent", "error-old", "completed-old"},
			expectedToKeep:  []string{},
			expectedOutput:  "Pruned 5 session(s).",
		},
		{
			name:            "prune --since 24h (does not prune running-old)",
			args:            []string{"prune", "--since", "24h"},
			expectedToPrune: []string{"error-old", "completed-old"},
			expectedToKeep:  []string{"running-recent", "running-old", "completed-recent"},
			expectedOutput:  "Pruned 2 session(s).",
		},
		{
			name:            "prune --all --since 24h (prunes running-old)",
			args:            []string{"prune", "--all", "--since", "24h"},
			expectedToPrune: []string{"running-old", "error-old", "completed-old"},
			expectedToKeep:  []string{"running-recent", "completed-recent"},
			expectedOutput:  "Pruned 3 session(s).",
		},
		{
			name:           "prune --dry-run with --since",
			args:           []string{"prune", "--dry-run", "--since", "24h"},
			expectedToKeep: []string{"running-recent", "running-old", "completed-recent", "error-old", "completed-old"}, // Nothing pruned
			expectedOutput: "Dry run enabled. The following sessions would be pruned:\n- error-old (status: error)\n- completed-old (status: completed)",
		},
		{
			name:           "prune --since 100h (no match)",
			args:           []string{"prune", "--since", "100h"},
			expectedToKeep: []string{"running-recent", "running-old", "completed-recent", "error-old", "completed-old"},
			expectedOutput: "No sessions to prune.",
		},
		{
			name:        "invalid since value",
			args:        []string{"prune", "--since", "invalid"},
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Reset session and log files before each test case.
			for _, s := range sessions {
				logPath := filepath.Join(sm.SessionsDir(), s.Name+".log")
				if _, err := os.Stat(logPath); os.IsNotExist(err) {
					require.NoError(t, os.WriteFile(logPath, []byte("log"), 0644))
				}
				require.NoError(t, sm.SaveSession(s))
			}

			// Execute the command. The test helper handles resetting flags.
			cmd := newPruneCmd()
			output, err := executeCommand(cmd, tc.args...)

			if tc.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "invalid duration format")
				return
			}
			require.NoError(t, err)

			// For multi-line expected output, check that each line is present, regardless of order.
			if strings.Contains(tc.expectedOutput, "\n") {
				expectedLines := strings.Split(tc.expectedOutput, "\n")
				for _, line := range expectedLines {
					assert.Contains(t, output, line)
				}
			} else {
				assert.Contains(t, output, tc.expectedOutput)
			}

			// Verify file system state after running the command.
			for _, sessionName := range tc.expectedToPrune {
				// The JSON file should be gone.
				_, err := sm.LoadSession(sessionName)
				assert.ErrorIs(t, err, os.ErrNotExist, "expected session %s to be pruned", sessionName)

				// The log file should be gone.
				logPath := filepath.Join(sm.SessionsDir(), sessionName+".log")
				_, err = os.Stat(logPath)
				assert.ErrorIs(t, err, os.ErrNotExist, "expected log for session %s to be pruned", sessionName)
			}

			for _, sessionName := range tc.expectedToKeep {
				// The JSON file should still exist.
				_, err := sm.LoadSession(sessionName)
				assert.NoError(t, err, "expected session %s to be kept", sessionName)

				// The log file should still exist.
				logPath := filepath.Join(sm.SessionsDir(), sessionName+".log")
				_, err = os.Stat(logPath)
				assert.NoError(t, err, "expected log for session %s to be kept", sessionName)
			}
		})
	}
}
