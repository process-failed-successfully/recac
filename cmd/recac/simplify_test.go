package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"recac/internal/agent"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockSimplifyAgent is a mock agent for simplify tests
type MockSimplifyAgent struct {
	mock.Mock
}

func (m *MockSimplifyAgent) Send(ctx context.Context, prompt string) (string, error) {
	args := m.Called(ctx, prompt)
	return args.String(0), args.Error(1)
}

func (m *MockSimplifyAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	args := m.Called(ctx, prompt, onChunk)
	response := args.String(0)
	onChunk(response)
	return response, args.Error(1)
}

func (m *MockSimplifyAgent) Close() error {
	m.Called()
	return nil
}

func TestSimplifyCmd_Stdout(t *testing.T) {
	// Create a temporary file
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.go")
	originalCode := "func ComplexFunction() { /* very complex */ }"
	err := os.WriteFile(filePath, []byte(originalCode), 0644)
	assert.NoError(t, err)

	// Setup mock agent
	mockAgent := new(MockSimplifyAgent)
	simplifiedCode := "func SimpleFunction() { /* simple */ }"
	mockAgent.On("Send", mock.Anything, mock.MatchedBy(func(prompt string) bool {
		return true
	})).Return("```go\n" + simplifiedCode + "\n```", nil)

	// Override factory
	oldFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}
	defer func() { agentClientFactory = oldFactory }()

	// Execute command via helper
	output, err := executeCommand(rootCmd, "simplify", filePath)
	assert.NoError(t, err)

	// Check output contains the simplified code
	assert.Contains(t, output, simplifiedCode)

	// Verify file was NOT modified
	content, err := os.ReadFile(filePath)
	assert.NoError(t, err)
	assert.Equal(t, originalCode, string(content))

	mockAgent.AssertExpectations(t)
}

func TestSimplifyCmd_InPlace(t *testing.T) {
	// Reset global flag to prevent state leakage
	defer func() { simplifyInPlace = false }()

	// Create a temporary file
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.go")
	originalCode := "func ComplexFunction() { /* very complex */ }"
	err := os.WriteFile(filePath, []byte(originalCode), 0644)
	assert.NoError(t, err)

	// Setup mock agent
	mockAgent := new(MockSimplifyAgent)
	simplifiedCode := "func SimpleFunction() { /* simple */ }"
	mockAgent.On("Send", mock.Anything, mock.Anything).Return(simplifiedCode, nil)

	// Override factory
	oldFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}
	defer func() { agentClientFactory = oldFactory }()

	// Execute via cobra command explicitly
	cmd := &cobra.Command{Use: "simplify", RunE: runSimplify}
	cmd.Flags().BoolVarP(&simplifyInPlace, "in-place", "i", false, "")
	cmd.SetArgs([]string{filePath, "--in-place"})

	err = cmd.Execute()
	assert.NoError(t, err)

	// Verify file WAS modified
	content, err := os.ReadFile(filePath)
	assert.NoError(t, err)
	assert.Equal(t, simplifiedCode, string(content))

	mockAgent.AssertExpectations(t)
}
