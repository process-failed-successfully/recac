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

// MockAgent for testing
type MockImproveAgent struct {
	mock.Mock
}

func (m *MockImproveAgent) Send(ctx context.Context, prompt string) (string, error) {
	args := m.Called(ctx, prompt)
	return args.String(0), args.Error(1)
}

func (m *MockImproveAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	args := m.Called(ctx, prompt, onChunk)
	return args.String(0), args.Error(1)
}

func TestImproveCmd(t *testing.T) {
	// Setup temporary directory
	tempDir, err := os.MkdirTemp("", "recac-improve-test")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create a dummy file to improve
	testFile := filepath.Join(tempDir, "test.go")
	originalCode := "package main\n\nfunc main() {\n\tprintln(\"hello\")\n}"
	err = os.WriteFile(testFile, []byte(originalCode), 0644)
	assert.NoError(t, err)

	improvedCode := "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"hello\")\n}"

	// Mock the agent factory
	origFactory := agentClientFactory
	defer func() { agentClientFactory = origFactory }()

	mockAgent := new(MockImproveAgent)
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}

	t.Run("Improve file (stdout)", func(t *testing.T) {
		mockAgent.On("Send", mock.Anything, mock.Anything).Return(improvedCode, nil).Once()

		cmd := NewImproveCmd()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(new(bytes.Buffer))

		// Run with file argument
		cmd.SetArgs([]string{testFile})
		err := cmd.Execute()
		assert.NoError(t, err)

		assert.Contains(t, buf.String(), "fmt.Println")
	})

	t.Run("Improve file in-place", func(t *testing.T) {
		mockAgent.On("Send", mock.Anything, mock.Anything).Return(improvedCode, nil).Once()

		cmd := NewImproveCmd()
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
		assert.Equal(t, improvedCode, string(content))
	})

	t.Run("Improve with diff", func(t *testing.T) {
		// Reset file content
		err = os.WriteFile(testFile, []byte(originalCode), 0644)
		assert.NoError(t, err)

		mockAgent.On("Send", mock.Anything, mock.Anything).Return(improvedCode, nil).Once()

		cmd := NewImproveCmd()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(new(bytes.Buffer))

		// Run with --diff
		cmd.SetArgs([]string{"--diff", testFile})
		err := cmd.Execute()
		assert.NoError(t, err)

		// Diff output should contain diff markers
		output := buf.String()
		assert.Contains(t, output, "---")
		assert.Contains(t, output, "+++")
		assert.Contains(t, output, "-	println(\"hello\")")
		assert.Contains(t, output, "+	fmt.Println(\"hello\")")
	})

	t.Run("CleanCode extracts from markdown", func(t *testing.T) {
		markdown := "Here is the code:\n```go\nfunc foo() {}\n```\nHope it helps."
		cleaned := cleanCode(markdown)
		assert.Equal(t, "func foo() {}", cleaned)
	})

	t.Run("CleanCode returns raw if no markdown", func(t *testing.T) {
		raw := "func foo() {}"
		cleaned := cleanCode(raw)
		assert.Equal(t, "func foo() {}", cleaned)
	})
}
