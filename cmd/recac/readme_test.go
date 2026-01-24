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

// ReadmeMockAgent unique name
type ReadmeMockAgent struct {
	mock.Mock
}

func (m *ReadmeMockAgent) Send(ctx context.Context, prompt string) (string, error) {
	args := m.Called(ctx, prompt)
	return args.String(0), args.Error(1)
}

func (m *ReadmeMockAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	args := m.Called(ctx, prompt, onChunk)
	return args.String(0), args.Error(1)
}

func TestReadmeCmd(t *testing.T) {
	// Setup temporary directory
	tempDir := t.TempDir()

	// Create dummy project files
	createDummyFile(t, tempDir, "go.mod", "module example.com/test")
	createDummyFile(t, tempDir, "main.go", "package main\nfunc main() {}")
	createDummyFile(t, tempDir, "subdir/ignored.go", "package ignored") // Should be in structure
	createDummyFile(t, tempDir, ".git/config", "ignored")               // Should be ignored

	// Mock Agent
	mockAgent := new(ReadmeMockAgent)
	expectedContent := "# Test Project\n\nThis is a generated readme."

	// We expect a call to Send with a prompt containing our file info
	mockAgent.On("Send", mock.Anything, mock.MatchedBy(func(prompt string) bool {
		// Check that the prompt contains key elements
		return assert.Contains(t, prompt, "Project Title & Description") &&
			assert.Contains(t, prompt, "main.go") &&
			assert.Contains(t, prompt, "module example.com/test")
	})).Return(expectedContent, nil)

	// Mock factory
	originalAgentFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}
	defer func() { agentClientFactory = originalAgentFactory }()

	// Reset flags
	readmeCmd.ResetFlags()
	// Re-init flags as ResetFlags clears them
	readmeCmd.Flags().StringP("out", "o", "README.md", "Output file path")

	// Set args
	outFile := filepath.Join(tempDir, "README_gen.md")
	// cmd/recac commands usually take args via SetArgs when using executeCommand helper,
	// but here we can just call RunE directly or use the helper if available.
	// We'll use a direct invocation approach for simplicity similar to how we'd call it from CLI logic.

	// Use executeCommand helper pattern if available, or manually set up cmd
	cmd := readmeCmd

	// Override flag for output file
	// runReadme reads flag from cmd.
	// We need to set the flag on the command instance.
	cmd.Flags().Set("out", outFile)

	// Execute
	err := runReadme(cmd, []string{tempDir}) // First arg is project path

	// Assert
	assert.NoError(t, err)

	// Check if file exists
	content, err := os.ReadFile(outFile)
	assert.NoError(t, err)
	assert.Equal(t, expectedContent, string(content))

	mockAgent.AssertExpectations(t)
}

func createDummyFile(t *testing.T, root, name, content string) {
	path := filepath.Join(root, name)
	err := os.MkdirAll(filepath.Dir(path), 0755)
	assert.NoError(t, err)
	err = os.WriteFile(path, []byte(content), 0644)
	assert.NoError(t, err)
}
