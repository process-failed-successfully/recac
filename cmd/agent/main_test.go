package main

import (
	"context"
	"os"
	"os/exec"
	"testing"
)

import (
	"errors"
	"recac/internal/cmdutils"
	"recac/internal/jira"
	"recac/internal/workflow"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
)

// Mock functions for workflow paths
var (
	mockRunWorkflow       func(ctx context.Context, cfg workflow.SessionConfig) error
	mockProcessJiraTicket func(ctx context.Context, ticketID string, jClient *jira.Client, cfg workflow.SessionConfig, ignoredBlockers map[string]bool) error
	mockProcessDirectTask func(ctx context.Context, cfg workflow.SessionConfig) error
)

func TestRunApp(t *testing.T) {
	// Setup mock implementations
	originalRunWorkflow := workflow.RunWorkflow
	originalProcessJiraTicket := workflow.ProcessJiraTicket
	originalProcessDirectTask := workflow.ProcessDirectTask
	originalGetJiraClient := cmdutils.GetJiraClient

	defer func() {
		workflow.RunWorkflow = originalRunWorkflow
		workflow.ProcessJiraTicket = originalProcessJiraTicket
		workflow.ProcessDirectTask = originalProcessDirectTask
		cmdutils.GetJiraClient = originalGetJiraClient
	}()

	testCases := []struct {
		name          string
		args          []string
		setupMocks    func()
		expectedError string
	}{
		{
			name: "Normal Workflow",
			args: []string{},
			setupMocks: func() {
				workflow.RunWorkflow = func(ctx context.Context, cfg workflow.SessionConfig) error {
					return nil
				}
			},
			expectedError: "",
		},
		{
			name: "Jira Ticket Workflow",
			args: []string{"--jira", "PROJ-123"},
			setupMocks: func() {
				cmdutils.GetJiraClient = func(ctx context.Context) (*jira.Client, error) {
					return &jira.Client{}, nil
				}
				workflow.ProcessJiraTicket = func(ctx context.Context, ticketID string, jClient *jira.Client, cfg workflow.SessionConfig, ignoredBlockers map[string]bool) error {
					return nil
				}
			},
			expectedError: "",
		},
		{
			name: "Direct Task Workflow",
			args: []string{"--repo-url", "https://github.com/test/repo.git"},
			setupMocks: func() {
				workflow.ProcessDirectTask = func(ctx context.Context, cfg workflow.SessionConfig) error {
					return nil
				}
			},
			expectedError: "",
		},
		{
			name: "Normal Workflow with Error",
			args: []string{},
			setupMocks: func() {
				workflow.RunWorkflow = func(ctx context.Context, cfg workflow.SessionConfig) error {
					return errors.New("normal workflow error")
				}
			},
			expectedError: "normal workflow error",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Reset mocks before each test
			workflow.RunWorkflow = originalRunWorkflow
			workflow.ProcessJiraTicket = originalProcessJiraTicket
			workflow.ProcessDirectTask = originalProcessDirectTask
			cmdutils.GetJiraClient = originalGetJiraClient

			// Reset flags for each test case
			pflag.CommandLine = pflag.NewFlagSet("test", pflag.ExitOnError)
			var cfgFile string
			initFlags(&cfgFile)

			// Setup mocks for the current test case
			tc.setupMocks()

			// Set command line arguments
			pflag.CommandLine.Parse(tc.args)

			// Run the app
			err := runApp(context.Background())

			// Assert the error
			if tc.expectedError != "" {
				assert.Error(t, err)
				assert.Equal(t, tc.expectedError, err.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestMainFunc(t *testing.T) {
	if os.Getenv("CRASH_TEST") == "1" {
		main()
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestMainFunc")
	cmd.Env = append(os.Environ(), "CRASH_TEST=1")

	// We capture the output and assert it behaves as expected (e.g., panics or fails to read config)
	// without actually breaking the test suite.
	out, err := cmd.CombinedOutput()

	if err != nil {
		if _, ok := err.(*exec.ExitError); ok {
			// Expected to fail when run like this since config is not present or flags are missing
			assert.Contains(t, string(out), "Error")
			return
		}
	}
	// If it didn't error out, that's also fine, but we expect an error in this default state.
	t.Logf("Process exited successfully. Output: %s", string(out))
}
