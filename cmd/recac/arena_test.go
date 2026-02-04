package main

import (
	"context"
	"io"
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
	t.Run("Run Arena Success", func(t *testing.T) {
		// Setup mock factory
		mockFactory := func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
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

		cmd := NewArenaCmd(mockFactory)
		// Set flags directly
		cmd.Flags().Set("competitors", "openai:gpt-4, gemini:gemini-pro")
		cmd.Flags().Set("task", "Who are you?")
		cmd.Flags().Set("judge-provider", "mock")
		cmd.Flags().Set("judge-model", "judge")

		// Silence output for tests
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)

		err := cmd.RunE(cmd, []string{})
		assert.NoError(t, err)
	})

	t.Run("Run Arena Validation Error", func(t *testing.T) {
		mockFactory := func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
			return &MockArenaAgent{Response: "OK"}, nil
		}
		cmd := NewArenaCmd(mockFactory)
		cmd.Flags().Set("competitors", "openai:gpt-4") // Only one

		// Silence output for tests
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)

		err := cmd.RunE(cmd, []string{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "requires at least 2 competitors")
	})

	t.Run("Run Arena With File Context", func(t *testing.T) {
		tmpDir := t.TempDir()
		tmpFile := tmpDir + "/context.txt"
		os.WriteFile(tmpFile, []byte("some context"), 0644)

		mockFactory := func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
			return &MockArenaAgent{Response: "OK"}, nil
		}

		cmd := NewArenaCmd(mockFactory)
		cmd.Flags().Set("competitors", "openai:gpt-4, gemini:gemini-pro")
		cmd.Flags().Set("task", "Read file")
		cmd.Flags().Set("file", tmpFile)
		cmd.Flags().Set("judge-provider", "mock")
		cmd.Flags().Set("judge-model", "judge")

		// Silence output for tests
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)

		err := cmd.RunE(cmd, []string{})
		assert.NoError(t, err)
	})
}
