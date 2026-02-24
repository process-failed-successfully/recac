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

// MockDoAgent is a mock implementation for testing Do command
type MockDoAgent struct {
	Response string
	Prompt   string
}

func (m *MockDoAgent) Send(ctx context.Context, prompt string) (string, error) {
	m.Prompt = prompt
	return m.Response, nil
}

func (m *MockDoAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	m.Prompt = prompt
	return m.Response, nil
}

// TestDoHelperProcess mocks exec.Command
func TestDoHelperProcess(t *testing.T) {
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

	// Handle sh -c "command"
	if cmd == "sh" && len(args) >= 3 && args[1] == "-c" {
		cmdStr := args[2]
		// Naive parsing for the test case "echo Hello World"
		if strings.HasPrefix(cmdStr, "echo ") {
			fmt.Fprint(os.Stdout, strings.TrimPrefix(cmdStr, "echo "))
			os.Exit(0)
		}
	}

	// Handle cmd /C "command" (Windows)
	if cmd == "cmd" && len(args) >= 3 && (args[1] == "/C" || args[1] == "/c") {
		cmdStr := args[2]
		if strings.HasPrefix(cmdStr, "echo ") {
			fmt.Fprint(os.Stdout, strings.TrimPrefix(cmdStr, "echo "))
			os.Exit(0)
		}
	}

	switch cmd {
	case "echo":
		fmt.Fprint(os.Stdout, "Hello World")
		os.Exit(0)
	default:
		fmt.Fprintf(os.Stderr, "Unknown mock command: %s args: %v\n", cmd, args)
		os.Exit(1)
	}
}

// Reuse or redefine executeCommandWithInput for test isolation
func executeDoCommandWithInput(root *cobra.Command, input string, args ...string) (output string, err error) {
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
	root.SetIn(bytes.NewBufferString(input))
	root.SetArgs(args)

	// Mock exit
	// Note: We don't need to mock exit here if we don't expect the command to call os.Exit
	// But recac commands often use RunE which returns error instead of exiting.
	// We'll trust Cobra's RunE behavior.

	err = root.Execute()
	output = b.String()
	return
}

func TestDoCmd_Success(t *testing.T) {
	// Setup Mock Exec
	defer func() { doExecCommand = exec.Command }()
	doExecCommand = func(command string, args ...string) *exec.Cmd {
		cs := []string{"-test.run=TestDoHelperProcess", "--", command}
		cs = append(cs, args...)
		cmd := exec.Command(os.Args[0], cs...)
		cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1"}
		return cmd
	}

	// Setup Mock Agent
	mockAgent := &MockDoAgent{
		Response: `{"command": "echo Hello World", "explanation": "Prints hello world"}`,
	}
	origFactory := agentClientFactory
	defer func() { agentClientFactory = origFactory }()
	agentClientFactory = func(ctx context.Context, p, m, d, n string) (agent.Agent, error) {
		return mockAgent, nil
	}

	// Execute with "y" confirmation
	output, err := executeDoCommandWithInput(rootCmd, "y\n", "do", "say hello")

	require.NoError(t, err)
	assert.Contains(t, output, "Explanation: Prints hello world")
	assert.Contains(t, output, "echo Hello World")
	assert.Contains(t, output, "Running...")
	assert.Contains(t, output, "Hello World") // Output from mock command

	// Verify Prompt contains OS/Shell info
	assert.Contains(t, mockAgent.Prompt, "You are a command line expert")
	assert.Contains(t, mockAgent.Prompt, "Instruction: \"say hello\"")
}

func TestDoCmd_Abort(t *testing.T) {
	// Setup Mock Agent
	mockAgent := &MockDoAgent{
		Response: `{"command": "rm -rf /", "explanation": "Deletes everything"}`,
	}
	origFactory := agentClientFactory
	defer func() { agentClientFactory = origFactory }()
	agentClientFactory = func(ctx context.Context, p, m, d, n string) (agent.Agent, error) {
		return mockAgent, nil
	}

	// Execute with "n" confirmation
	output, err := executeDoCommandWithInput(rootCmd, "n\n", "do", "delete everything")

	require.NoError(t, err)
	assert.Contains(t, output, "Deletes everything")
	assert.Contains(t, output, "Execute this command? [y/N]")
	assert.Contains(t, output, "Aborted")
	assert.NotContains(t, output, "Running...")
}

func TestDoCmd_InvalidJSON(t *testing.T) {
	// Setup Mock Agent
	mockAgent := &MockDoAgent{
		Response: `Invalid JSON`,
	}
	origFactory := agentClientFactory
	defer func() { agentClientFactory = origFactory }()
	agentClientFactory = func(ctx context.Context, p, m, d, n string) (agent.Agent, error) {
		return mockAgent, nil
	}

	// Execute
	output, err := executeDoCommandWithInput(rootCmd, "y\n", "do", "something")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse agent response")
	assert.Contains(t, output, "Thinking...")
}

func TestDoCmd_ConversationalResponse(t *testing.T) {
	// Setup Mock Exec
	defer func() { doExecCommand = exec.Command }()
	doExecCommand = func(command string, args ...string) *exec.Cmd {
		cs := []string{"-test.run=TestDoHelperProcess", "--", command}
		cs = append(cs, args...)
		cmd := exec.Command(os.Args[0], cs...)
		cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1"}
		return cmd
	}

	// Setup Mock Agent
	mockAgent := &MockDoAgent{
		Response: `Sure, here is the command you requested:
		` + "```json" + `
		{
			"command": "echo Conversational",
			"explanation": "Extracts JSON from text"
		}
		` + "```" + `
		Hope this helps!`,
	}
	origFactory := agentClientFactory
	defer func() { agentClientFactory = origFactory }()
	agentClientFactory = func(ctx context.Context, p, m, d, n string) (agent.Agent, error) {
		return mockAgent, nil
	}

	// Execute with "y" confirmation
	output, err := executeDoCommandWithInput(rootCmd, "y\n", "do", "extract json")

	require.NoError(t, err)
	assert.Contains(t, output, "Extracts JSON from text")
	assert.Contains(t, output, "echo Conversational")
}
