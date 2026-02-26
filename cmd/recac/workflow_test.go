package main

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"testing"

	"recac/internal/agent"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

// MockWorkflowAgent mocks the agent.Agent interface
type MockWorkflowAgent struct {
	Response string
	Error    error
}

func (m *MockWorkflowAgent) Send(ctx context.Context, prompt string) (string, error) {
	return m.Response, m.Error
}

func (m *MockWorkflowAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	return m.Response, m.Error
}

func TestWorkflowCommand(t *testing.T) {
	// Backup and Restore Factories
	originalAgentFactory := agentClientFactory
	originalExecCommand := execCommand
	defer func() {
		agentClientFactory = originalAgentFactory
		execCommand = originalExecCommand
	}()

	// Mock Agent Response
	mockResponse := `{
		"title": "Test Workflow",
		"steps": [
			{ "command": "echo step1", "description": "Running step 1" },
			{ "command": "echo step2", "description": "Running step 2" }
		]
	}`

	agentClientFactory = func(ctx context.Context, provider, model, path, project string) (agent.Agent, error) {
		return &MockWorkflowAgent{Response: mockResponse}, nil
	}

	// Mock Exec Command to capture execution
	var executedCommands []string
	execCommand = func(name string, arg ...string) *exec.Cmd {
		// Capture the full command line "sh -c <cmd>"
		if len(arg) > 1 && arg[0] == "-c" {
			executedCommands = append(executedCommands, arg[1])
		} else {
			executedCommands = append(executedCommands, name+" "+fmt.Sprint(arg))
		}
		// Return a successful dummy command (echo)
		return exec.Command("echo", "mock")
	}

	// Setup Command
	root := &cobra.Command{Use: "testroot"}
	root.AddCommand(workflowCmd)

	// Create buffers for input/output
	var outBuf, errBuf bytes.Buffer
	var inBuf bytes.Buffer
	inBuf.WriteString("y\n") // Auto-confirm

	root.SetOut(&outBuf)
	root.SetErr(&errBuf)
	root.SetIn(&inBuf)

	// Run "workflow do something"
	root.SetArgs([]string{"workflow", "do", "something"})
	err := root.Execute()

	// Assertions
	assert.NoError(t, err)
	output := outBuf.String()
	assert.Contains(t, output, "Plan: Test Workflow")
	assert.Contains(t, output, "Running step 1")
	assert.Contains(t, output, "Running step 2")

	if assert.Len(t, executedCommands, 2) {
		assert.Equal(t, "echo step1", executedCommands[0])
		assert.Equal(t, "echo step2", executedCommands[1])
	}
}
