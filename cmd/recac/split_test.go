package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"recac/internal/agent"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockAgentForSplit is a mock implementation of the Agent interface for Split tests.
type MockAgentForSplit struct {
	mock.Mock
}

func (m *MockAgentForSplit) Send(ctx context.Context, prompt string) (string, error) {
	args := m.Called(ctx, prompt)
	return args.String(0), args.Error(1)
}

func (m *MockAgentForSplit) SendStream(ctx context.Context, prompt string, callback func(string)) (string, error) {
	args := m.Called(ctx, prompt, callback)
	return args.String(0), args.Error(1)
}

func TestSplitCmd(t *testing.T) {
	tempDir := t.TempDir()

	// 1. Create a dummy file to split
	targetFile := filepath.Join(tempDir, "big.go")
	content := `package main

	type User struct {}

	func (u *User) Do() {}
	`
	err := os.WriteFile(targetFile, []byte(content), 0644)
	assert.NoError(t, err)

	// 2. Mock Agent
	mockAgent := new(MockAgentForSplit)

	originalAgentFactory := agentClientFactory
	defer func() {
		agentClientFactory = originalAgentFactory
	}()

	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}

	mockJSONResponse := `
	{
		"types.go": "package main\n\ntype User struct {}",
		"handlers.go": "package main\n\nfunc (u *User) Do() {}"
	}`

	mockAgent.On("Send", mock.Anything, mock.MatchedBy(func(prompt string) bool {
		return true // accept any prompt
	})).Return(mockJSONResponse, nil)

	// 3. Execute Split Cmd
	root, _, _ := newRootCmd()
	output, err := executeCommand(root, "split", targetFile, "--delete")
	assert.NoError(t, err)

	// 4. Verify Output and Side Effects
	assert.Contains(t, output, "Created")
	assert.Contains(t, output, "Deleted original file")

	// Ensure new files exist
	typesContent, err := os.ReadFile(filepath.Join(tempDir, "types.go"))
	assert.NoError(t, err)
	assert.Contains(t, string(typesContent), "type User struct {}")

	handlersContent, err := os.ReadFile(filepath.Join(tempDir, "handlers.go"))
	assert.NoError(t, err)
	assert.Contains(t, string(handlersContent), "func (u *User) Do() {}")

	// Ensure old file is deleted
	_, err = os.Stat(targetFile)
	assert.True(t, os.IsNotExist(err), "Original file should have been deleted")

	mockAgent.AssertExpectations(t)
}
