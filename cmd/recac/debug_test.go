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

func TestExtractFileContexts_EdgeCases(t *testing.T) {
	t.Run("Phantom file", func(t *testing.T) {
		// Output references a file that doesn't exist
		out := "phantom.go:10: error"
		context, err := extractFileContexts(out)
		assert.NoError(t, err)
		assert.Equal(t, "Files referenced in output could not be read.", context)
	})

	t.Run("Large file truncation", func(t *testing.T) {
		// Create a large file
		filename := "large.go"
		content := strings.Repeat("a", 10*1024+100) // > 10KB
		err := os.WriteFile(filename, []byte(content), 0644)
		assert.NoError(t, err)
		defer os.Remove(filename)

		out := filename + ":1: error"
		ctx, err := extractFileContexts(out)
		assert.NoError(t, err)
		assert.Contains(t, ctx, "... (truncated)")
		assert.Less(t, len(ctx), len(content))
	})

	t.Run("File read error", func(t *testing.T) {
		// Create a file with no read permissions
		filename := "locked.go"
		err := os.WriteFile(filename, []byte("secret"), 0000) // No permissions
		// Skip if root (root can read 0000 files usually)
		if os.Geteuid() == 0 {
			os.Remove(filename)
			t.Skip("Skipping permission test as root")
		}
		assert.NoError(t, err)
		defer func() {
			os.Chmod(filename, 0644) // cleanup needs write
			os.Remove(filename)
		}()

		out := filename + ":1: error"
		ctx, err := extractFileContexts(out)
		assert.NoError(t, err)
		// Should just skip the file or report error in context
		// The code says: sb.WriteString(fmt.Sprintf("Could not read file %s: %v\n", path, err))
		assert.Contains(t, ctx, "Could not read file")
	})
}
