package main

import (
	"os"
	"os/exec"
	"testing"

	"recac/internal/runner"
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
				// Extract from args for tests that should fail before this matters
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
			var capturedArgs []string
			// Override the package-level variable with a mock function
			execCommand = func(executable string, args ...string) *exec.Cmd {
				capturedArgs = args
				// Return a dummy command that does nothing when Run() is called
				return exec.Command("true")
			}
			// Restore the original function after the test
			defer func() { execCommand = exec.Command }()

			// Override the session manager factory
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
			} else {
				require.NoError(t, err)

				// Verify the arguments passed to the mocked exec.Command
				require.NotNil(t, capturedArgs, "execCommand was not called")

				// Check for --resume-from flag
				foundResume := false
				for i, arg := range capturedArgs {
					if arg == "--resume-from" {
						require.True(t, i+1 < len(capturedArgs), "missing value for --resume-from")
						require.Equal(t, tc.expectedResumeArg, capturedArgs[i+1])
						foundResume = true
						break
					}
				}
				require.True(t, foundResume, "did not find --resume-from flag")

				// Check for --name flag
				foundName := false
				for i, arg := range capturedArgs {
					if arg == "--name" {
						require.True(t, i+1 < len(capturedArgs), "missing value for --name")
						require.Equal(t, tc.expectedNameArg, capturedArgs[i+1])
						foundName = true
						break
					}
				}
				require.True(t, foundName, "did not find --name flag")

				// Check if original flags were preserved (e.g., --detached)
				if contains(tc.originalArgs, "--detached") {
					require.True(t, contains(capturedArgs, "--detached"), "did not preserve --detached flag")
				}
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
