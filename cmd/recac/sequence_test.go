package main

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"

	"recac/internal/agent"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockAgentSequence is a mock implementation of agent.Agent for sequence tests.
type MockAgentSequence struct {
	mock.Mock
}

func (m *MockAgentSequence) Send(ctx context.Context, prompt string) (string, error) {
	args := m.Called(ctx, prompt)
	return args.String(0), args.Error(1)
}

func (m *MockAgentSequence) SendStream(ctx context.Context, prompt string, callback func(string)) (string, error) {
	args := m.Called(ctx, prompt, callback)
	if callback != nil {
		callback(args.String(0))
	}
	return args.String(0), args.Error(1)
}

func TestSequenceCmd(t *testing.T) {
	// Restore factory after test
	originalFactory := agentClientFactory
	defer func() { agentClientFactory = originalFactory }()

	mockAgent := new(MockAgentSequence)
	agentClientFactory = func(ctx context.Context, provider, model, dir, id string) (agent.Agent, error) {
		return mockAgent, nil
	}

	expectedMermaid := "sequenceDiagram\n    participant A\n    participant B\n    A->>B: Hello"

	// Expect Send call with prompt containing scenario
	mockAgent.On("Send", mock.Anything, mock.MatchedBy(func(prompt string) bool {
		return strings.Contains(prompt, "Scenario: \"User logs in\"") &&
		       strings.Contains(prompt, "Code Context:")
	})).Return(expectedMermaid, nil)

	// Setup command
	// We need to reset flags because they are global
	sequenceOutput = ""
	sequenceFocus = []string{"."}
	sequenceIgnore = nil

	cmd := &cobra.Command{
		Use: "sequence",
	}
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	err := runSequence(cmd, []string{"User logs in"})
	assert.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, expectedMermaid)

	mockAgent.AssertExpectations(t)
}

func TestSequenceCmd_WithOutput(t *testing.T) {
	// Restore factory
	originalFactory := agentClientFactory
	defer func() { agentClientFactory = originalFactory }()

	mockAgent := new(MockAgentSequence)
	agentClientFactory = func(ctx context.Context, provider, model, dir, id string) (agent.Agent, error) {
		return mockAgent, nil
	}

	expectedMermaid := "sequenceDiagram\n    A->>B: Hi"
	mockAgent.On("Send", mock.Anything, mock.Anything).Return(expectedMermaid, nil)

	// Temp file
	tmpOutput := "test_sequence.mmd"
	defer os.Remove(tmpOutput)

	// Set global flags
	sequenceOutput = tmpOutput
	sequenceFocus = []string{"."}
	defer func() { sequenceOutput = "" }() // Reset

	cmd := &cobra.Command{Use: "sequence"}
	cmd.SetOut(new(bytes.Buffer))

	err := runSequence(cmd, []string{"Test Output"})
	assert.NoError(t, err)

	// Check file exists
	content, err := os.ReadFile(tmpOutput)
	assert.NoError(t, err)
	assert.Equal(t, expectedMermaid, string(content))

	mockAgent.AssertExpectations(t)
}

func TestSequenceCmd_AgentMarkdownStrip(t *testing.T) {
	originalFactory := agentClientFactory
	defer func() { agentClientFactory = originalFactory }()

	mockAgent := new(MockAgentSequence)
	agentClientFactory = func(ctx context.Context, provider, model, dir, id string) (agent.Agent, error) {
		return mockAgent, nil
	}

	rawResponse := "Here is the diagram:\n```mermaid\nsequenceDiagram\n    A->>B: Hi\n```"
	expectedMermaid := "sequenceDiagram\n    A->>B: Hi"

	mockAgent.On("Send", mock.Anything, mock.Anything).Return(rawResponse, nil)

	sequenceOutput = ""
	sequenceFocus = []string{"."}

	cmd := &cobra.Command{Use: "sequence"}
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	err := runSequence(cmd, []string{"Test Strip"})
	assert.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, expectedMermaid)
	assert.NotContains(t, output, "Here is the diagram")
	assert.NotContains(t, output, "```mermaid")
}
