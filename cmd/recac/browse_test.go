package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFormatGitURLToHTTPS(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "SSH URL",
			input:    "git@github.com:user/repo.git",
			expected: "https://github.com/user/repo",
		},
		{
			name:     "HTTPS URL",
			input:    "https://github.com/user/repo.git",
			expected: "https://github.com/user/repo",
		},
		{
			name:     "HTTPS URL without .git",
			input:    "https://github.com/user/repo",
			expected: "https://github.com/user/repo",
		},
		{
			name:     "SSH URL with hyphens",
			input:    "git@gitlab.com:my-org/my-project.git",
			expected: "https://gitlab.com/my-org/my-project",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := formatGitURLToHTTPS(tt.input)
			assert.Equal(t, tt.expected, actual)
		})
	}
}

func TestBrowseCmd(t *testing.T) {
	originalGitClientFactory := gitClientFactory
	defer func() {
		gitClientFactory = originalGitClientFactory
	}()

	originalBrowseExecCommand := browseExecCommand
	defer func() {
		browseExecCommand = originalBrowseExecCommand
	}()

	tests := []struct {
		name           string
		mockRepoExists bool
		mockRemoteURL  string
		mockRunError   error
		expectError    bool
		expectedOutput string
		expectedExec   string
	}{
		{
			name:           "Not a git repository",
			mockRepoExists: false,
			expectError:    true,
			expectedOutput: "Error: not a git repository\n",
		},
		{
			name:           "No origin remote",
			mockRepoExists: true,
			mockRemoteURL:  "",
			mockRunError:   fmt.Errorf("no remote origin"),
			expectError:    true,
			expectedOutput: "Error: could not find remote 'origin' URL\n",
		},
		{
			name:           "Success with SSH URL",
			mockRepoExists: true,
			mockRemoteURL:  "git@github.com:user/repo.git\n",
			mockRunError:   nil,
			expectError:    false,
			expectedOutput: "Opened https://github.com/user/repo in the browser.\n",
			expectedExec:   "https://github.com/user/repo",
		},
		{
			name:           "Success with HTTPS URL",
			mockRepoExists: true,
			mockRemoteURL:  "https://github.com/user/repo.git\n",
			mockRunError:   nil,
			expectError:    false,
			expectedOutput: "Opened https://github.com/user/repo in the browser.\n",
			expectedExec:   "https://github.com/user/repo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gitClientFactory = func() IGitClient {
				return &MockGitClient{
					RepoExistsFunc: func(repoPath string) bool {
						return tt.mockRepoExists
					},
					RunFunc: func(repoPath string, args ...string) (string, error) {
						if len(args) == 3 && args[0] == "remote" && args[1] == "get-url" && args[2] == "origin" {
							return tt.mockRemoteURL, tt.mockRunError
						}
						return "", fmt.Errorf("unexpected command")
					},
				}
			}

			// Mock execution process to capture command and prevent opening browser
			var capturedExecArgs []string
			browseExecCommand = func(name string, arg ...string) *exec.Cmd {
				capturedExecArgs = append([]string{name}, arg...)
				// Use the helper process pattern to avoid actually running `open` or `xdg-open`
				cs := []string{"-test.run=TestBrowseHelperProcess", "--", name}
				cs = append(cs, arg...)
				cmd := exec.Command(os.Args[0], cs...)
				cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1"}
				return cmd
			}

			cmd := browseCmd
			outBuf := new(bytes.Buffer)
			// Reset the command execution properties to avoid side-effects
			cmd.SetOut(outBuf)
			cmd.SetErr(outBuf)
			cmd.SetArgs([]string{}) // no args needed for browse
			// We need to call the RunE method directly because cmd.Execute()
			// parses flags globally and might trigger root help text if not setup correctly.
			err := cmd.RunE(cmd, []string{})
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Contains(t, outBuf.String(), tt.expectedOutput)

				// Check if the exec arguments contained the expected formatted URL
				if tt.expectedExec != "" {
					joinedArgs := strings.Join(capturedExecArgs, " ")
					assert.Contains(t, joinedArgs, tt.expectedExec)
				}
			}
		})
	}
}

func TestBrowseHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	defer os.Exit(0)
	// We simply exit 0 to indicate the browser opened successfully.
}
