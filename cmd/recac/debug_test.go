package main

import (
	"context"
	"os"
	"strings"
	"testing"
	"recac/internal/agent"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// DebugMockAgent is a unique mock to avoid collisions
type DebugMockAgent struct {
	mock.Mock
}

func (m *DebugMockAgent) Send(ctx context.Context, prompt string) (string, error) {
	args := m.Called(ctx, prompt)
	return args.String(0), args.Error(1)
}

func (m *DebugMockAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	args := m.Called(ctx, prompt, onChunk)
	// Simulate streaming
	if onChunk != nil {
		onChunk(args.String(0))
	}
	return args.String(0), args.Error(1)
}

func TestDebugCmd_Success(t *testing.T) {
	// Command that succeeds
	out, err := executeCommand(rootCmd, "debug", "echo hello")
	assert.NoError(t, err)
	assert.Contains(t, out, "hello")
	assert.NotContains(t, out, "Analyzing failure")
}

func TestDebugCmd_Failure(t *testing.T) {
	// Mock agent factory
	origFactory := agentClientFactory
	defer func() { agentClientFactory = origFactory }()

	mockAgent := new(DebugMockAgent)
	mockAgent.On("SendStream", mock.Anything, mock.Anything, mock.Anything).Return("Try checking the syntax.", nil)

	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}

	// Command that fails
	out, err := executeCommand(rootCmd, "debug", "echo 'fail me'; exit 1")

	// Since the agent successfully runs, the command returns nil (success of the debug tool)
	assert.NoError(t, err)
	assert.Contains(t, out, "fail me")
	assert.Contains(t, out, "Analyzing failure")
	assert.Contains(t, out, "Try checking the syntax")
}

func TestDebugCmd_WithFiles(t *testing.T) {
	// Create a dummy file
	err := os.WriteFile("dummy.go", []byte("package main\n\nfunc foo() {}"), 0644)
	assert.NoError(t, err)
	defer os.Remove("dummy.go")

	// Mock agent
	origFactory := agentClientFactory
	defer func() { agentClientFactory = origFactory }()

	mockAgent := new(DebugMockAgent)
	// Matcher to verify file content is in prompt
	mockAgent.On("SendStream", mock.Anything, mock.MatchedBy(func(prompt string) bool {
		return strings.Contains(prompt, "dummy.go") && strings.Contains(prompt, "func foo()")
	}), mock.Anything).Return("Fixing dummy.", nil)

	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}

	// Command that fails and references the file
	// We mimic output like "dummy.go:3: error"
	cmdStr := "echo 'dummy.go:3: error'; exit 1"
	out, err := executeCommand(rootCmd, "debug", cmdStr)

	assert.NoError(t, err)
	assert.Contains(t, out, "Fixing dummy")
}
