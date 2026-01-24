package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"testing"

	"recac/internal/agent"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockRunAgent is a mock implementation of the Agent interface for testing run command
type MockRunAgent struct {
	CapturedPrompt string
	Response       string
}

func (m *MockRunAgent) Send(ctx context.Context, prompt string) (string, error) {
	m.CapturedPrompt = prompt
	return m.Response, nil
}

func (m *MockRunAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	m.CapturedPrompt = prompt
	onChunk(m.Response)
	return m.Response, nil
}

// TestRunCmdHelperProcess is used to mock os/exec.Command
// It is not a real test, but a helper invoked by the test process.
func TestRunCmdHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	defer os.Exit(0)

	// Extract the actual command and args passed after "--"
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
	// Handle different mock scenarios
	switch cmd {
	case "success_cmd":
		fmt.Fprint(os.Stdout, "Command succeeded")
		os.Exit(0)
	case "fail_cmd":
		fmt.Fprint(os.Stdout, "Partial output before failure")
		fmt.Fprint(os.Stderr, "Command failed with error")
		os.Exit(1)
	default:
		fmt.Fprintf(os.Stderr, "Unknown mock command: %s\n", cmd)
		os.Exit(2)
	}
}

func TestRunCmd_Success(t *testing.T) {
	// Restore original execCommand after test
	defer func() { runExecCommand = exec.Command }()

	// Mock execCommand to call TestRunCmdHelperProcess
	runExecCommand = func(command string, args ...string) *exec.Cmd {
		cs := []string{"-test.run=TestRunCmdHelperProcess", "--", command}
		cs = append(cs, args...)
		cmd := exec.Command(os.Args[0], cs...)
		cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1"}
		return cmd
	}

	// We don't expect the agent to be called
	mockAgent := &MockRunAgent{}
	// Override factory
	origFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, p, m, d, n string) (agent.Agent, error) {
		return mockAgent, nil
	}
	defer func() { agentClientFactory = origFactory }()

	// Execute
	output, err := executeCommand(rootCmd, "run", "success_cmd")

	// Assertions
	require.NoError(t, err)
	assert.Contains(t, output, "Command succeeded")
	assert.NotContains(t, output, "Asking AI for help")
	assert.Empty(t, mockAgent.CapturedPrompt)
}

func TestRunCmd_Failure_CallsAI(t *testing.T) {
	// Restore original execCommand after test
	defer func() { runExecCommand = exec.Command }()

	// Mock execCommand to call TestRunCmdHelperProcess
	runExecCommand = func(command string, args ...string) *exec.Cmd {
		cs := []string{"-test.run=TestRunCmdHelperProcess", "--", command}
		cs = append(cs, args...)
		cmd := exec.Command(os.Args[0], cs...)
		cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1"}
		return cmd
	}

	// Mock Agent
	mockAgent := &MockRunAgent{
		Response: "The command failed because you mocked it to fail.",
	}
	// Override factory
	origFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, p, m, d, n string) (agent.Agent, error) {
		return mockAgent, nil
	}
	defer func() { agentClientFactory = origFactory }()

	// Execute
	// We expect an error because the command fails
	output, err := executeCommand(rootCmd, "run", "fail_cmd")

	// Assertions
	require.Error(t, err) // It should return an error
	assert.Contains(t, output, "Partial output before failure") // Stdout should be captured
	assert.Contains(t, output, "Command failed with error") // Stderr should be captured
	assert.Contains(t, output, "Asking AI for help")
	assert.Contains(t, output, "The command failed because you mocked it to fail.")

	// Check prompt content
	assert.Contains(t, mockAgent.CapturedPrompt, "<command>\nfail_cmd")
	assert.Contains(t, mockAgent.CapturedPrompt, "<output>\nPartial output before failure")
	assert.Contains(t, mockAgent.CapturedPrompt, "Command failed with error")
}
