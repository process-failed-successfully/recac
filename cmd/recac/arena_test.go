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

		// Reset flags
		arenaCmd.Flags().Set("competitors", "")
		arenaCmd.Flags().Set("task", "")
		arenaCmd.Flags().Set("file", "")
		arenaCmd.Flags().Set("judge-provider", "")
		arenaCmd.Flags().Set("judge-model", "")

		// Set flags
		err := arenaCmd.Flags().Set("competitors", "openai:gpt-4, gemini:gemini-pro")
		assert.NoError(t, err)
		err = arenaCmd.Flags().Set("task", "Who are you?")
		assert.NoError(t, err)
		err = arenaCmd.Flags().Set("judge-provider", "mock")
		assert.NoError(t, err)
		err = arenaCmd.Flags().Set("judge-model", "judge")
		assert.NoError(t, err)

		err = runArena(arenaCmd, []string{})
		assert.NoError(t, err)
	})

	t.Run("Run Arena Validation Error", func(t *testing.T) {
		// Reset flags
		arenaCmd.Flags().Set("competitors", "")
		arenaCmd.Flags().Set("task", "")

		err := arenaCmd.Flags().Set("competitors", "openai:gpt-4") // Only one
		assert.NoError(t, err)

		err = runArena(arenaCmd, []string{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "requires at least 2 competitors")
	})

    t.Run("Run Arena With File Context", func(t *testing.T) {
        tmpDir := t.TempDir()
        tmpFile := tmpDir + "/context.txt"
        os.WriteFile(tmpFile, []byte("some context"), 0644)

		// Reset flags
		arenaCmd.Flags().Set("competitors", "")
		arenaCmd.Flags().Set("task", "")
		arenaCmd.Flags().Set("file", "")
		arenaCmd.Flags().Set("judge-provider", "")
		arenaCmd.Flags().Set("judge-model", "")

        err := arenaCmd.Flags().Set("competitors", "openai:gpt-4, gemini:gemini-pro")
		assert.NoError(t, err)
        err = arenaCmd.Flags().Set("task", "Read file")
		assert.NoError(t, err)
        err = arenaCmd.Flags().Set("file", tmpFile)
		assert.NoError(t, err)
        err = arenaCmd.Flags().Set("judge-provider", "mock")
		assert.NoError(t, err)
        err = arenaCmd.Flags().Set("judge-model", "judge")
		assert.NoError(t, err)

        agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
             return &MockArenaAgent{Response: "OK"}, nil
        }

        err = runArena(arenaCmd, []string{})
        assert.NoError(t, err)
    })
}
