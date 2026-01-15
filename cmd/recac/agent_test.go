package main

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

type mockExecutor struct {
	args []string
}

func (m *mockExecutor) Run() error {
	return nil
}

func TestRunAgentCommand(t *testing.T) {
	origCommandExecutor := commandExecutor
	defer func() { commandExecutor = origCommandExecutor }()

	var capturedArgs []string
	commandExecutor = func(command string, args ...string) Executor {
		capturedArgs = args
		return &mockExecutor{args: args}
	}

	// Test cases
	testCases := []struct {
		name     string
		args     []string
		expected []string
	}{
		{
			name:     "Jira and Verbose",
			args:     []string{"--jira=PROJ-123", "-v"},
			expected: []string{"--jira=PROJ-123", "-v"},
		},
		{
			name:     "Repo URL and Summary",
			args:     []string{"--repo-url=http://a.com", "--summary=Test"},
			expected: []string{"--repo-url=http://a.com", "--summary=Test"},
		},
		{
			name:     "All Flags",
			args:     []string{"--jira=PROJ-123", "--repo-url=http://a.com", "--summary=Test", "--verbose=true", "--stream=false", "--allow-dirty=true", "--image=test-image", "--provider=test-provider", "--model=test-model", "--mock=true", "--max-iterations=50"},
			expected: []string{"--jira=PROJ-123", "--repo-url=http://a.com", "--summary=Test", "-v", "--stream=false", "--allow-dirty=true", "--image=test-image", "--provider=test-provider", "--model=test-model", "--mock=true", "--max-iterations=50"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			testRootCmd, _, _ := newTestAgentCmd()
			capturedArgs = nil // Reset captured args
			testRootCmd.SetArgs(append([]string{"agent", "run"}, tc.args...))
			err := testRootCmd.Execute()
			assert.NoError(t, err)

			// Sort slices for consistent comparison
			assert.ElementsMatch(t, tc.expected, capturedArgs)
		})
	}
}

func newTestAgentCmd() (*cobra.Command, *bytes.Buffer, *bytes.Buffer) {
	testRootCmd := &cobra.Command{Use: "recac"}
	testAgentCmd := &cobra.Command{Use: "agent"}
	testRunCmd := &cobra.Command{
		Use: "run",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAgent(cmd, args)
		},
	}

	testRunCmd.Flags().String("jira", "", "Jira Ticket ID")
	testRunCmd.Flags().String("repo-url", "", "Repository URL")
	testRunCmd.Flags().String("summary", "", "Task summary")
	testRunCmd.Flags().BoolP("verbose", "v", false, "Enable verbose logging")
	testRunCmd.Flags().Bool("stream", true, "Stream agent output")
	testRunCmd.Flags().Bool("allow-dirty", false, "Allow dirty git status")
	testRunCmd.Flags().String("image", "ghcr.io/process-failed-successfully/recac-agent:latest", "Agent Docker image")
	testRunCmd.Flags().String("provider", "", "AI provider")
	testRunCmd.Flags().String("model", "", "AI model")
	testRunCmd.Flags().Bool("mock", false, "Enable mock mode")
	testRunCmd.Flags().Int("max-iterations", 30, "Maximum iterations")

	testAgentCmd.AddCommand(testRunCmd)
	testRootCmd.AddCommand(testAgentCmd)

	var out, errOut bytes.Buffer
	testRootCmd.SetOut(&out)
	testRootCmd.SetErr(&errOut)

	return testRootCmd, &out, &errOut
}

func TestVerboseFlag(t *testing.T) {
	origCommandExecutor := commandExecutor
	defer func() { commandExecutor = origCommandExecutor }()

	var capturedArgs []string
	commandExecutor = func(command string, args ...string) Executor {
		capturedArgs = args
		return &mockExecutor{args: args}
	}

	testCases := []struct {
		name          string
		args          []string
		expectVerbose bool
	}{
		{name: "Short verbose flag", args: []string{"-v"}, expectVerbose: true},
		{name: "Long verbose flag", args: []string{"--verbose"}, expectVerbose: true},
		{name: "Verbose flag with value", args: []string{"--verbose=true"}, expectVerbose: true},
		{name: "No verbose flag", args: []string{}, expectVerbose: false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			testRootCmd, _, _ := newTestAgentCmd()
			capturedArgs = nil
			testRootCmd.SetArgs(append([]string{"agent", "run"}, tc.args...))
			err := testRootCmd.Execute()
			assert.NoError(t, err)

			found := false
			for _, arg := range capturedArgs {
				if arg == "-v" || arg == "--verbose=true" {
					found = true
					break
				}
			}
			assert.Equal(t, tc.expectVerbose, found, "Verbose flag mismatch")
		})
	}
}
