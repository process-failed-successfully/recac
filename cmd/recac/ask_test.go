package main

import (
	"bytes"
	"context"
	"recac/internal/agent"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockAgentClient is a mock implementation of agent.Agent
type MockAgentClient struct {
	mock.Mock
}

func (m *MockAgentClient) Send(ctx context.Context, prompt string) (string, error) {
	args := m.Called(ctx, prompt)
	return args.String(0), args.Error(1)
}

func (m *MockAgentClient) SendStream(ctx context.Context, prompt string, callback func(string)) (string, error) {
	args := m.Called(ctx, prompt, callback)
	callback(args.String(0)) // Simulate streaming
	return args.String(0), args.Error(1)
}

func TestAskCmd(t *testing.T) {
	// Mock agent factory
	originalFactory := agentClientFactory
	defer func() { agentClientFactory = originalFactory }()

	mockAgent := new(MockAgentClient)
	agentClientFactory = func(ctx context.Context, provider, model, dir, id string) (agent.Agent, error) {
		return mockAgent, nil
	}

	// Mock agent response
	expectedAnswer := "The answer is 42."
	mockAgent.On("SendStream", mock.Anything, mock.Anything, mock.Anything).Return(expectedAnswer, nil)

	// Execute command
	// Create a new command to avoid rootCmd interference
	cmd := &cobra.Command{
		Use:  "ask",
		RunE: runAskCmd,
	}

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	// We pass args to Execute() differently when creating a fresh command?
	// No, SetArgs parses flags. But here we are setting RunE directly.
	// We need to simulate arguments.
	// If we use cmd.Execute(), it parses os.Args if SetArgs is not called.
	cmd.SetArgs([]string{"What is the meaning of life?"})

	// We also need to make sure `askIgnore` and `askMaxSize` are available/reset if they are package-level vars.
	// They are, so we should reset them or set them via flags if we were testing flags.
	// For this test, we are testing the main logic.

	err := cmd.Execute()
	assert.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "Analyzing codebase...")
	assert.Contains(t, output, "Consulting Agent...")
	assert.Contains(t, output, expectedAnswer)

	// Verify the prompt contained the question
	mockAgent.AssertCalled(t, "SendStream", mock.Anything, mock.MatchedBy(func(prompt string) bool {
		return strings.Contains(prompt, "QUESTION:\nWhat is the meaning of life?") &&
			strings.Contains(prompt, "CONTEXT:")
	}), mock.Anything)
}
