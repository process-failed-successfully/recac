package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"recac/internal/agent"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockMonitorAgent is a mock implementation of the Agent interface
type MockMonitorAgent struct {
	CapturedPrompt string
	Response       string
	SendStreamCalled bool
}

func (m *MockMonitorAgent) Send(ctx context.Context, prompt string) (string, error) {
	m.CapturedPrompt = prompt
	return m.Response, nil
}

func (m *MockMonitorAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	m.CapturedPrompt = prompt
	m.SendStreamCalled = true
	onChunk(m.Response)
	return m.Response, nil
}

// TestMonitorHelperProcess is used to mock os/exec.Command
func TestMonitorHelperProcess(t *testing.T) {
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
	switch cmd {
	case "success_cmd":
		fmt.Fprintln(os.Stdout, "Command succeeded")
		os.Exit(0)
	case "fail_cmd":
		fmt.Fprintln(os.Stdout, "Doing some work...")
		time.Sleep(100 * time.Millisecond)
		fmt.Fprintln(os.Stderr, "Error: Something went wrong")
		time.Sleep(100 * time.Millisecond)
		os.Exit(1)
	case "panic_cmd":
		fmt.Fprintln(os.Stdout, "Everything is fine...")
		time.Sleep(100 * time.Millisecond)
		fmt.Fprintln(os.Stdout, "panic: index out of range")
		time.Sleep(100 * time.Millisecond)
		os.Exit(2)
	}
}

func TestMonitorCmd_Success(t *testing.T) {
	// Mock execCommand
	monitorExecCommand = func(command string, args ...string) *exec.Cmd {
		cs := []string{"-test.run=TestMonitorHelperProcess", "--", command}
		cs = append(cs, args...)
		cmd := exec.Command(os.Args[0], cs...)
		cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1"}
		return cmd
	}
	defer func() { monitorExecCommand = exec.Command }()

	// Mock Agent
	mockAgent := &MockMonitorAgent{}
	origFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, p, m, d, n string) (agent.Agent, error) {
		return mockAgent, nil
	}
	defer func() { agentClientFactory = origFactory }()

	// Capture output
	cmd := monitorCmd
	buf := new(strings.Builder)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	// Execute
	monitorCmd.RunE(cmd, []string{"success_cmd"})

	// Assertions
	output := buf.String()
	assert.Contains(t, output, "Command succeeded")
	assert.NotContains(t, output, "Analyzing error")
	assert.False(t, mockAgent.SendStreamCalled)
}

func TestMonitorCmd_ErrorTrigger(t *testing.T) {
	// Mock execCommand
	monitorExecCommand = func(command string, args ...string) *exec.Cmd {
		cs := []string{"-test.run=TestMonitorHelperProcess", "--", command}
		cs = append(cs, args...)
		cmd := exec.Command(os.Args[0], cs...)
		cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1"}
		return cmd
	}
	defer func() { monitorExecCommand = exec.Command }()

	// Mock Agent
	mockAgent := &MockMonitorAgent{
		Response: "Fix the error by checking logs.",
	}
	origFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, p, m, d, n string) (agent.Agent, error) {
		return mockAgent, nil
	}
	defer func() { agentClientFactory = origFactory }()

	// Capture output
	cmd := monitorCmd
	buf := new(strings.Builder)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	// Execute
	// We expect RunE to return error because the command exits with 1
	err := monitorCmd.RunE(cmd, []string{"fail_cmd"})

	// Wait a bit for async goroutines to finish (RunE waits for waitgroup, so it should be fine)

	// Assertions
	require.Error(t, err)
	output := buf.String()
	assert.Contains(t, output, "Doing some work...")
	assert.Contains(t, output, "Error: Something went wrong")
	assert.Contains(t, output, "Analyzing error...")
	assert.Contains(t, output, "Fix the error by checking logs.")
	assert.True(t, mockAgent.SendStreamCalled)

	assert.Contains(t, mockAgent.CapturedPrompt, "Error: Something went wrong")
}

func TestMonitorCmd_PanicTrigger(t *testing.T) {
	// Mock execCommand
	monitorExecCommand = func(command string, args ...string) *exec.Cmd {
		cs := []string{"-test.run=TestMonitorHelperProcess", "--", command}
		cs = append(cs, args...)
		cmd := exec.Command(os.Args[0], cs...)
		cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1"}
		return cmd
	}
	defer func() { monitorExecCommand = exec.Command }()

	// Mock Agent
	mockAgent := &MockMonitorAgent{
		Response: "Fix the panic.",
	}
	origFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, p, m, d, n string) (agent.Agent, error) {
		return mockAgent, nil
	}
	defer func() { agentClientFactory = origFactory }()

	// Capture output
	cmd := monitorCmd
	buf := new(strings.Builder)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	// Execute
	err := monitorCmd.RunE(cmd, []string{"panic_cmd"})

	// Assertions
	require.Error(t, err)
	output := buf.String()
	assert.Contains(t, output, "panic: index out of range")
	assert.Contains(t, output, "Analyzing error...")
	assert.Contains(t, output, "Fix the panic.")
	assert.True(t, mockAgent.SendStreamCalled)

	assert.Contains(t, mockAgent.CapturedPrompt, "panic: index out of range")
}
