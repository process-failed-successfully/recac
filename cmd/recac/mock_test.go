package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"recac/internal/agent"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type DummyMockAgent struct {
	Response string
}

func (m *DummyMockAgent) Send(ctx context.Context, prompt string) (string, error) {
	return m.Response, nil
}

func (m *DummyMockAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	return m.Response, nil
}

func (m *DummyMockAgent) GetState() interface{} {
	return nil
}

func TestMockCmd(t *testing.T) {
	// Setup temp dir and temp go file
	tmpDir, err := os.MkdirTemp("", "mock_cmd_test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	srcFile := filepath.Join(tmpDir, "dummy.go")
	srcContent := `package main

type MyInterface interface {
	Do() error
}
`
	err = os.WriteFile(srcFile, []byte(srcContent), 0644)
	require.NoError(t, err)

	// Mock Agent Client Factory
	originalAgentFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return &DummyMockAgent{
			Response: "package main\n\ntype MyInterfaceMock struct{}\n",
		}, nil
	}
	defer func() { agentClientFactory = originalAgentFactory }()

	// Reset flags and bind mockCmd
	resetFlags(mockCmd)

	// We use the executeCommand helper if available or standard Execute
	// Set arguments for mockCmd and run it directly
	err = runMock(mockCmd, []string{srcFile})
	assert.NoError(t, err)

	// Check the generated output file
	expectedOutput := filepath.Join(tmpDir, "myinterface_mock.go")
	content, err := os.ReadFile(expectedOutput)
	assert.NoError(t, err)
	assert.Contains(t, string(content), "type MyInterfaceMock struct{}")
}
