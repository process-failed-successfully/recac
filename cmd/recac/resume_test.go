package main

import (
	"os"
	"testing"

	"recac/internal/runner"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func TestResumeCommand(t *testing.T) {
	// --- Test Cases ---
	tests := []struct {
		name              string
		sessionStatus     string
		originalArgs      []string
		expectError       bool
		expectedErrorMsg  string
		expectedResumeArg string // The workspace path expected in --resume-from
		expectedNameArg   string // The name expected in --name
	}{
		{
			name:              "Resume an errored session",
			sessionStatus:     "error",
			originalArgs:      []string{"start", "--path", "/original/ws", "--name", "errored-session"},
			expectError:       false,
			expectedResumeArg: "/original/ws",
			expectedNameArg:   "errored-session",
		},
		{
			name:              "Resume a stopped session",
			sessionStatus:     "stopped",
			originalArgs:      []string{"start", "--path", "/original/ws", "--name", "stopped-session"},
			expectError:       false,
			expectedResumeArg: "/original/ws",
			expectedNameArg:   "stopped-session",
		},
		{
			name:             "Fail to resume a running session",
			sessionStatus:    "running",
			originalArgs:     []string{"start", "--name", "running-session"},
			expectError:      true,
			expectedErrorMsg: "is already running",
		},
		{
			name:              "Resume command preserves original flags",
			sessionStatus:     "error",
			originalArgs:      []string{"start", "--path", "/ws/special", "--name", "special-session", "--detached", "--max-iterations", "50"},
			expectError:       false,
			expectedResumeArg: "/ws/special",
			expectedNameArg:   "special-session",
		},
		{
			name:             "Fail on non-resumable status",
			sessionStatus:    "pending",
			originalArgs:     []string{"start", "--name", "pending-session"},
			expectError:      true,
			expectedErrorMsg: "cannot be resumed",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// --- Setup ---
			tempDir := t.TempDir()
			sm, err := runner.NewSessionManagerWithDir(tempDir)
			require.NoError(t, err)

			sessionName := tc.expectedNameArg
			if sessionName == "" {
				for i, arg := range tc.originalArgs {
					if arg == "--name" && i+1 < len(tc.originalArgs) {
						sessionName = tc.originalArgs[i+1]
						break
					}
				}
			}

			mockSession := &runner.SessionState{
				Name:      sessionName,
				Status:    tc.sessionStatus,
				Command:   append([]string{"recac"}, tc.originalArgs...),
				Workspace: tc.expectedResumeArg,
				PID:       0,
			}
			if tc.sessionStatus == "running" {
				mockSession.PID = os.Getpid() // Use a real PID
			}
			err = sm.SaveSession(mockSession)
			require.NoError(t, err)

			// --- Mocking ---
			var startCmdCalled bool
			originalStartRun := startCmd.Run
			startCmd.Run = func(cmd *cobra.Command, args []string) {
				startCmdCalled = true
				// Verify flags passed to the start command
				resumeFrom, _ := cmd.Flags().GetString("resume-from")
				require.Equal(t, tc.expectedResumeArg, resumeFrom)

				name, _ := cmd.Flags().GetString("name")
				require.Equal(t, tc.expectedNameArg, name)

				if contains(tc.originalArgs, "--detached") {
					detached, _ := cmd.Flags().GetBool("detached")
					require.True(t, detached)
				}
				if contains(tc.originalArgs, "--max-iterations") {
					maxIter, _ := cmd.Flags().GetInt("max-iterations")
					require.Equal(t, 50, maxIter) // Hardcoded from test case
				}
			}
			defer func() { startCmd.Run = originalStartRun }()

			// Override the session manager factory to use our mock
			sessionManagerFactory = func() (ISessionManager, error) {
				return sm, nil
			}

			// --- Execution ---
			rootCmd, _, _ := newRootCmd()
			rootCmd.SetArgs([]string{"resume", sessionName})
			err = rootCmd.Execute()

			// --- Assertions ---
			if tc.expectError {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.expectedErrorMsg)
				require.False(t, startCmdCalled, "start command should not be called on error")
			} else {
				// The resume command itself doesn't return an error, it executes the start command.
				// If startCmd.Run panics on an assertion, the test will fail, which is correct.
				// If it returns an error, Execute() will propagate it.
				require.NoError(t, err)
				require.True(t, startCmdCalled, "start command was not called")
			}
		})
	}
}

// contains is a helper to check for string presence in a slice.
func contains(slice []string, str string) bool {
	for _, v := range slice {
		if v == str {
			return true
		}
	}
	return false
}
