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

// MockAgent for diff-explain tests
type MockDiffExplainAgent struct {
	mock.Mock
}

func (m *MockDiffExplainAgent) Send(ctx context.Context, prompt string) (string, error) {
	args := m.Called(ctx, prompt)
	return args.String(0), args.Error(1)
}

func (m *MockDiffExplainAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	args := m.Called(ctx, prompt, onChunk)
	// Simulate streaming
	response := args.String(0)
	onChunk(response)
	return response, args.Error(1)
}

func (m *MockDiffExplainAgent) Close() error {
	return nil
}

func TestDiffExplainCmd_Stdin(t *testing.T) {
	// Setup mock agent
	mockAgent := new(MockDiffExplainAgent)
	mockAgent.On("SendStream", mock.Anything, mock.MatchedBy(func(prompt string) bool {
		return strings.Contains(prompt, "diff --git a/file.go")
	}), mock.Anything).Return("This is an explanation of the diff.", nil)

	// Override factory
	oldFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}
	defer func() { agentClientFactory = oldFactory }()

	// Override checkPipedInput
	oldCheck := checkPipedInput
	checkPipedInput = func() bool { return false } // We use Buffer so this defaults to false, but our code handles Buffer
	defer func() { checkPipedInput = oldCheck }()

	// Create command
	cmd := &cobra.Command{Use: "diff-explain", RunE: runDiffExplain}

	// Mock Stdin with a buffer
	inputDiff := "diff --git a/file.go b/file.go\nindex 123..456 100644\n--- a/file.go\n+++ b/file.go\n@@ -1 +1 @@\n-foo\n+bar"
	buf := bytes.NewBufferString(inputDiff)
	cmd.SetIn(buf)

	// Capture output
	outBuf := new(bytes.Buffer)
	cmd.SetOut(outBuf)

	// Run
	err := cmd.Execute()
	assert.NoError(t, err)

	// Verify output
	output := outBuf.String()
	assert.Contains(t, output, "This is an explanation of the diff.")
	mockAgent.AssertExpectations(t)
}

func TestDiffExplainCmd_EmptyStdin(t *testing.T) {
	// Setup command
	cmd := &cobra.Command{Use: "diff-explain", RunE: runDiffExplain}

	// Mock empty Stdin
	buf := bytes.NewBufferString("")
	cmd.SetIn(buf)

	// Capture output
	outBuf := new(bytes.Buffer)
	cmd.SetOut(outBuf)

	// Run
	err := cmd.Execute()
	assert.NoError(t, err)

	// Verify output
	output := outBuf.String()
	assert.Contains(t, output, "No changes detected")
}
