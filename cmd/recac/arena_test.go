package main

import (
	"context"
	"os"
	"recac/internal/agent"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// MockArenaAgent is a mock agent for testing the arena
type MockArenaAgent struct {
	Response string
}

func (m *MockArenaAgent) Send(ctx context.Context, prompt string) (string, error) {
	// Simulate some latency
	time.Sleep(10 * time.Millisecond)
	return m.Response, nil
}

func (m *MockArenaAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	return m.Send(ctx, prompt)
}

func TestArenaCmd(t *testing.T) {
	// Save original factory
	originalFactory := agentClientFactory
	defer func() {
		agentClientFactory = originalFactory
	}()

	// Ensure flags are reset after the test
	defer resetFlags(arenaCmd)

	t.Run("Run Arena Success", func(t *testing.T) {
		// Setup mock factory
		agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
			if strings.Contains(projectName, "recac-arena-judge") {
				return &MockArenaAgent{
					Response: "WINNER: Candidate 1\nREASONING: Better response.",
				}, nil
			}

			// Competitors
			if provider == "openai" && model == "gpt-4" {
				return &MockArenaAgent{Response: "I am GPT-4"}, nil
			}
			if provider == "gemini" && model == "gemini-pro" {
				return &MockArenaAgent{Response: "I am Gemini"}, nil
			}

			return &MockArenaAgent{Response: "Unknown"}, nil
		}

		// Use executeCommand with flags on rootCmd
		output, err := executeCommand(rootCmd, "arena", "--competitors", "openai:gpt-4, gemini:gemini-pro", "--task", "Who are you?", "--judge-provider", "mock", "--judge-model", "judge")
		assert.NoError(t, err)
		assert.Contains(t, output, "WINNER: Candidate 1")
		assert.Contains(t, output, "finished in")
	})

	t.Run("Run Arena Validation Error", func(t *testing.T) {
		_, err := executeCommand(rootCmd, "arena", "--competitors", "openai:gpt-4", "--task", "Test")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "requires at least 2 competitors")
	})

    t.Run("Run Arena With File Context", func(t *testing.T) {
        tmpDir := t.TempDir()
        tmpFile := tmpDir + "/context.txt"
        os.WriteFile(tmpFile, []byte("some context"), 0644)

        agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
             return &MockArenaAgent{Response: "OK"}, nil
        }

        output, err := executeCommand(rootCmd, "arena", "--competitors", "openai:gpt-4, gemini:gemini-pro", "--task", "Read file", "--file", tmpFile, "--judge-provider", "mock", "--judge-model", "judge")
        assert.NoError(t, err)
        assert.Contains(t, output, "Starting Arena")
    })
}
