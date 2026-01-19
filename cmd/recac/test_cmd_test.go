package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"recac/internal/agent"
)

// Mock helper for exec.Command using the existing TestHelperProcess in commands_test.go
// The existing TestHelperProcess runs rootCmd.Execute().
// This is tricky because we want to intercept 'go test' calls, not 'recac' calls.
// The existing helper is designed for integration tests of recac commands.

// We need a specific helper for our unit test mocking of exec.Command.
// To avoid name collision, we will use a different name for the helper function and match it via args.

func mockExecCommandHelperForTestCmd(command string, args ...string) *exec.Cmd {
	cs := []string{"-test.run=TestHelperProcessForTestCmd", "--", command}
	cs = append(cs, args...)
	cmd := exec.Command(os.Args[0], cs...)
	cmd.Env = []string{"GO_WANT_HELPER_PROCESS_FOR_TEST_CMD=1"}
	return cmd
}

// TestHelperProcessForTestCmd is our custom helper
func TestHelperProcessForTestCmd(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS_FOR_TEST_CMD") != "1" {
		return
	}
	defer os.Exit(0)

	// Skip the test runner arguments
	args := os.Args
	for len(args) > 0 {
		if args[0] == "--" {
			args = args[1:]
			break
		}
		args = args[1:]
	}

	if len(args) == 0 {
		os.Exit(0)
	}

	cmd := args[0]
	if cmd == "go" && len(args) > 1 && args[1] == "test" {
		// Simulation for go test
		for _, arg := range args {
			if arg == "fail_pkg" {
				fmt.Println("FAIL: TestSomething")
				fmt.Println("FAIL")
				os.Exit(1)
			}
		}
		fmt.Println("ok package 0.1s")
		os.Exit(0)
	}
}

// Local mock agent
type CmdMockAgent struct {
	Response string
}

func (m *CmdMockAgent) Send(ctx context.Context, prompt string) (string, error) {
	if m.Response == "" {
		return "Default mock response", nil
	}
	return m.Response, nil
}

func (m *CmdMockAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	resp, _ := m.Send(ctx, prompt)
	if onChunk != nil {
		onChunk(resp)
	}
	return resp, nil
}

func TestTestCmd_Success(t *testing.T) {
	// Mock execCommand
	oldExec := testExecCommand
	testExecCommand = mockExecCommandHelperForTestCmd
	defer func() { testExecCommand = oldExec }()

	// Mock Agent Factory
	oldFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, cwd, agentID string) (agent.Agent, error) {
		return &CmdMockAgent{}, nil
	}
	defer func() { agentClientFactory = oldFactory }()

	cmd := NewTestCmd()
	var outBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetErr(&outBuf)

	cmd.SetArgs([]string{"pass_pkg"})
	err := cmd.Execute()

	assert.NoError(t, err)
	assert.Contains(t, outBuf.String(), "ok package")
	assert.Contains(t, outBuf.String(), "All tests passed")
}

func TestTestCmd_Failure_Analysis(t *testing.T) {
	// Mock execCommand
	oldExec := testExecCommand
	testExecCommand = mockExecCommandHelperForTestCmd
	defer func() { testExecCommand = oldExec }()

	// Mock Agent
	mockAg := &CmdMockAgent{
		Response: "Here is the fix for the failure.",
	}
	oldFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, cwd, agentID string) (agent.Agent, error) {
		return mockAg, nil
	}
	defer func() { agentClientFactory = oldFactory }()

	cmd := NewTestCmd()
	var outBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetErr(&outBuf)

	cmd.SetArgs([]string{"fail_pkg"})
	err := cmd.Execute()

	assert.Error(t, err) // Should return error
	assert.Contains(t, outBuf.String(), "FAIL")
	assert.Contains(t, outBuf.String(), "Analyzing with AI")
	assert.Contains(t, outBuf.String(), "Here is the fix")
}
