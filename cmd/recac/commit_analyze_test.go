package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"

	"recac/internal/agent"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockCommitAnalyzeAgent is a mock implementation for testing CommitAnalyze command
type MockCommitAnalyzeAgent struct {
	Response string
	Prompt   string
	Error    error
}

func (m *MockCommitAnalyzeAgent) Send(ctx context.Context, prompt string) (string, error) {
	m.Prompt = prompt
	if m.Error != nil {
		return "", m.Error
	}
	return m.Response, nil
}

func (m *MockCommitAnalyzeAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	m.Prompt = prompt
	if m.Error != nil {
		return "", m.Error
	}
	onChunk(m.Response)
	return m.Response, nil
}

// TestCommitAnalyzeHelperProcess mocks exec.Command for git
func TestCommitAnalyzeHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	defer os.Exit(0)

	args := os.Args
	for len(args) > 0 {
		if args[0] == "--" {
			args = args[1:]
			break
		}
		args = args[1:]
	}

	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "No command provided to helper process\n")
		os.Exit(2)
	}

	cmd := args[0]
	if cmd != "git" {
		fmt.Fprintf(os.Stderr, "Unknown mock command: %s args: %v\n", cmd, args)
		os.Exit(1)
	}

	if len(args) >= 3 && args[1] == "show" {
		commitHash := args[2]
		if commitHash == "error-hash" {
			fmt.Fprintf(os.Stderr, "fatal: bad object error-hash\n")
			os.Exit(1)
		}
		if commitHash == "empty-hash" {
			fmt.Fprint(os.Stdout, "")
			os.Exit(0)
		}

		// return dummy diff
		fmt.Fprint(os.Stdout, `commit `+commitHash+`
Author: Mock User <mock@example.com>
Date:   Wed May 1 12:00:00 2024 -0400

    Add sample feature
`)
		os.Exit(0)
	}

	fmt.Fprintf(os.Stderr, "Unknown git command: %v\n", args)
	os.Exit(1)
}

func executeCommitAnalyzeCommand(root *cobra.Command, args ...string) (output string, err error) {
	// Reset flags
	root.Flags().VisitAll(func(f *pflag.Flag) {
		if f.Changed {
			if strings.Contains(strings.ToLower(f.Value.Type()), "slice") {
				f.Value.Set("")
			} else {
				f.Value.Set(f.DefValue)
			}
			f.Changed = false
		}
	})

	b := new(bytes.Buffer)

	// Capture output
	root.SetOut(b)
	root.SetErr(b)
	root.SetArgs(args)

	err = root.Execute()
	output = b.String()
	return
}

func TestCommitAnalyzeCmd_Success(t *testing.T) {
	// Setup Mock Exec
	defer func() { execCommand = exec.Command }()
	execCommand = func(command string, args ...string) *exec.Cmd {
		cs := []string{"-test.run=TestCommitAnalyzeHelperProcess", "--", command}
		cs = append(cs, args...)
		cmd := exec.Command(os.Args[0], cs...)
		cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1"}
		return cmd
	}

	// Setup Mock Agent
	mockAgent := &MockCommitAnalyzeAgent{
		Response: "Looks good. Great commit.",
	}
	origFactory := agentClientFactory
	defer func() { agentClientFactory = origFactory }()
	agentClientFactory = func(ctx context.Context, p, m, d, n string) (agent.Agent, error) {
		return mockAgent, nil
	}

	output, err := executeCommitAnalyzeCommand(rootCmd, "commit-analyze", "HEAD")

	require.NoError(t, err)
	assert.Contains(t, output, "Fetching diff for commit: HEAD...")
	assert.Contains(t, output, "Analyzing commit...")
	assert.Contains(t, output, "Looks good. Great commit.")
	assert.Contains(t, mockAgent.Prompt, "Add sample feature")
}

func TestCommitAnalyzeCmd_GitError(t *testing.T) {
	// Setup Mock Exec
	defer func() { execCommand = exec.Command }()
	execCommand = func(command string, args ...string) *exec.Cmd {
		cs := []string{"-test.run=TestCommitAnalyzeHelperProcess", "--", command}
		cs = append(cs, args...)
		cmd := exec.Command(os.Args[0], cs...)
		cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1"}
		return cmd
	}

	output, err := executeCommitAnalyzeCommand(rootCmd, "commit-analyze", "error-hash")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to fetch git diff for commit error-hash")
	assert.NotContains(t, output, "Analyzing commit...")
}

func TestCommitAnalyzeCmd_EmptyCommit(t *testing.T) {
	// Setup Mock Exec
	defer func() { execCommand = exec.Command }()
	execCommand = func(command string, args ...string) *exec.Cmd {
		cs := []string{"-test.run=TestCommitAnalyzeHelperProcess", "--", command}
		cs = append(cs, args...)
		cmd := exec.Command(os.Args[0], cs...)
		cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1"}
		return cmd
	}

	output, err := executeCommitAnalyzeCommand(rootCmd, "commit-analyze", "empty-hash")

	require.NoError(t, err)
	assert.Contains(t, output, "Commit has no changes or does not exist")
	assert.NotContains(t, output, "Analyzing commit...")
}

func TestCommitAnalyzeCmd_AgentFailure(t *testing.T) {
	// Setup Mock Exec
	defer func() { execCommand = exec.Command }()
	execCommand = func(command string, args ...string) *exec.Cmd {
		cs := []string{"-test.run=TestCommitAnalyzeHelperProcess", "--", command}
		cs = append(cs, args...)
		cmd := exec.Command(os.Args[0], cs...)
		cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1"}
		return cmd
	}

	// Setup Mock Agent that returns an error
	mockAgent := &MockCommitAnalyzeAgent{
		Error: fmt.Errorf("AI is down"),
	}
	origFactory := agentClientFactory
	defer func() { agentClientFactory = origFactory }()
	agentClientFactory = func(ctx context.Context, p, m, d, n string) (agent.Agent, error) {
		return mockAgent, nil
	}

	output, err := executeCommitAnalyzeCommand(rootCmd, "commit-analyze", "HEAD")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "agent failed during analysis: AI is down")
	assert.Contains(t, output, "Analyzing commit...")
}
