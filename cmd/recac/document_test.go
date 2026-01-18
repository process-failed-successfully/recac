package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"recac/internal/agent"
)

// MockDocumentAgent for testing
type MockDocumentAgent struct {
	mock.Mock
}

func (m *MockDocumentAgent) Send(ctx context.Context, prompt string) (string, error) {
	args := m.Called(ctx, prompt)
	return args.String(0), args.Error(1)
}

func (m *MockDocumentAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	args := m.Called(ctx, prompt, onChunk)
	return args.String(0), args.Error(1)
}

func TestDocumentCmd(t *testing.T) {
	// Setup temporary directory
	tempDir, err := os.MkdirTemp("", "recac-document-test")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create a dummy file to document
	testFile := filepath.Join(tempDir, "test.go")
	originalCode := "package main\n\nfunc main() {\n\tprintln(\"hello\")\n}"
	err = os.WriteFile(testFile, []byte(originalCode), 0644)
	assert.NoError(t, err)

	documentedCode := "package main\n\n// main prints hello\nfunc main() {\n\tprintln(\"hello\")\n}"

	// Mock the agent factory
	origFactory := agentClientFactory
	defer func() { agentClientFactory = origFactory }()

	mockAgent := new(MockDocumentAgent)
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}

	t.Run("Document file (stdout)", func(t *testing.T) {
		mockAgent.On("Send", mock.Anything, mock.Anything).Return(documentedCode, nil).Once()

		cmd := NewDocumentCmd()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(new(bytes.Buffer))

		// Run with file argument
		cmd.SetArgs([]string{testFile})
		err := cmd.Execute()
		assert.NoError(t, err)

		assert.Contains(t, buf.String(), "// main prints hello")
	})

	t.Run("Document file in-place", func(t *testing.T) {
		mockAgent.On("Send", mock.Anything, mock.Anything).Return(documentedCode, nil).Once()

		cmd := NewDocumentCmd()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(new(bytes.Buffer))

		// Run with --in-place
		cmd.SetArgs([]string{"--in-place", testFile})
		err := cmd.Execute()
		assert.NoError(t, err)

		// Verify file content changed
		content, err := os.ReadFile(testFile)
		assert.NoError(t, err)
		assert.Equal(t, documentedCode, string(content))
	})

	t.Run("Document stdin", func(t *testing.T) {
		mockAgent.On("Send", mock.Anything, mock.Anything).Return(documentedCode, nil).Once()

		cmd := NewDocumentCmd()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(new(bytes.Buffer))
		cmd.SetIn(bytes.NewBufferString(originalCode))

		// Run with no args (stdin)
		cmd.SetArgs([]string{})
		err := cmd.Execute()
		assert.NoError(t, err)

		assert.Contains(t, buf.String(), "// main prints hello")
	})

	t.Run("CleanCodeFromMarkdown extracts from markdown", func(t *testing.T) {
		markdown := "Here is the code:\n```go\nfunc foo() {}\n```\nHope it helps."
		cleaned := cleanCodeFromMarkdown(markdown)
		assert.Equal(t, "func foo() {}", cleaned)
	})
}
