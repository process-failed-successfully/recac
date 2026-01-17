package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"recac/internal/agent"
	"testing"

	"github.com/stretchr/testify/assert"
)

type MockExplainAgent struct {
	Response      string
	StreamChunks  []string
	Err           error
	LastPrompt    string
}

func (m *MockExplainAgent) Send(ctx context.Context, prompt string) (string, error) {
	m.LastPrompt = prompt
	return m.Response, m.Err
}

func (m *MockExplainAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	m.LastPrompt = prompt
	if m.Err != nil {
		return "", m.Err
	}
	for _, chunk := range m.StreamChunks {
		onChunk(chunk)
	}
	return m.Response, nil
}

func TestExplainCmd(t *testing.T) {
	// Restore factory after tests
	originalFactory := agentClientFactory
	defer func() { agentClientFactory = originalFactory }()

	t.Run("File input", func(t *testing.T) {
		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "test.go")
		err := os.WriteFile(filePath, []byte("package main\nfunc main() {}"), 0644)
		assert.NoError(t, err)

		mockAgent := &MockExplainAgent{
			Response:     "This is a main function.",
			StreamChunks: []string{"This ", "is ", "a ", "main ", "function."},
		}

		agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
			return mockAgent, nil
		}

		cmd := NewExplainCmd()
		var stdout, stderr bytes.Buffer
		cmd.SetOut(&stdout)
		cmd.SetErr(&stderr)
		cmd.SetArgs([]string{filePath})

		err = cmd.Execute()
		assert.NoError(t, err)
		assert.Contains(t, stdout.String(), "This is a main function.")
		assert.Contains(t, stderr.String(), "Analyzing code...")
		assert.Contains(t, mockAgent.LastPrompt, "package main")
	})

	t.Run("Stdin input", func(t *testing.T) {
		mockAgent := &MockExplainAgent{
			Response:     "Stdin explanation",
			StreamChunks: []string{"Stdin ", "explanation"},
		}

		agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
			return mockAgent, nil
		}

		cmd := NewExplainCmd()
		var stdout, stderr bytes.Buffer
		var stdin bytes.Buffer
		stdin.WriteString("some code from stdin")

		cmd.SetOut(&stdout)
		cmd.SetErr(&stderr)
		cmd.SetIn(&stdin)
		cmd.SetArgs([]string{}) // No file arg

		err := cmd.Execute()
		assert.NoError(t, err)
		assert.Contains(t, stdout.String(), "Stdin explanation")
		assert.Contains(t, mockAgent.LastPrompt, "some code from stdin")
	})

	t.Run("Empty input", func(t *testing.T) {
		cmd := NewExplainCmd()
		var stdout, stderr bytes.Buffer
		var stdin bytes.Buffer // Empty stdin

		cmd.SetOut(&stdout)
		cmd.SetErr(&stderr)
		cmd.SetIn(&stdin)
		cmd.SetArgs([]string{})

		err := cmd.Execute()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "input is empty")
	})

	t.Run("File read error", func(t *testing.T) {
		cmd := NewExplainCmd()
		cmd.SetArgs([]string{"non-existent-file.go"})
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)

		err := cmd.Execute()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to read file")
	})

	t.Run("Agent error", func(t *testing.T) {
		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "test.go")
		os.WriteFile(filePath, []byte("code"), 0644)

		agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
			return nil, errors.New("agent creation failed")
		}

		cmd := NewExplainCmd()
		cmd.SetArgs([]string{filePath})
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)

		err := cmd.Execute()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create agent")
	})
}
