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
	t.Parallel()

	t.Run("Run Arena Success", func(t *testing.T) {
		t.Parallel()

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
		// Discard output to avoid log clutter and race on stdout (though print is thread safe, discard is cleaner)
		// But in this test we might want to verify output? The original test didn't verify output content beyond "no error".
		// To fix race on global stdout/stderr if any, setting output is good.
		// cmd.SetOut(io.Discard)
		// cmd.SetErr(io.Discard)
		// Actually, let's capture it to debug if needed, or discard.
		// Since we parallelize, we should definitely NOT write to real os.Stdout if possible, although Go test handles it.
		// Best practice: capture.

		// Use a buffer if we wanted to assert. For now, io.Discard to ensure no race.
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)

		args := []string{
			"--competitors", "openai:gpt-4, gemini:gemini-pro",
			"--task", "Who are you?",
			"--judge-provider", "mock",
			"--judge-model", "judge",
		}
		err := cmd.ParseFlags(args)
		assert.NoError(t, err)
		err = cmd.RunE(cmd, cmd.Flags().Args())
		assert.NoError(t, err)
	})

	t.Run("Run Arena Validation Error", func(t *testing.T) {
		t.Parallel()

		mockFactory := func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
			return &MockArenaAgent{Response: "OK"}, nil
		}

		cmd := NewArenaCmd(mockFactory)
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)

		args := []string{
			"--competitors", "openai:gpt-4", // Only one
			"--task", "Who are you?",
		}
		err := cmd.ParseFlags(args)
		assert.NoError(t, err)
		err = cmd.RunE(cmd, cmd.Flags().Args())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "requires at least 2 competitors")
	})

	t.Run("Run Arena With File Context", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		tmpFile := tmpDir + "/context.txt"
		os.WriteFile(tmpFile, []byte("some context"), 0644)

		mockFactory := func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
			return &MockArenaAgent{Response: "OK"}, nil
		}

		cmd := NewArenaCmd(mockFactory)
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)

		args := []string{
			"--competitors", "openai:gpt-4, gemini:gemini-pro",
			"--task", "Read file",
			"--file", tmpFile,
			"--judge-provider", "mock",
			"--judge-model", "judge",
		}
		err := cmd.ParseFlags(args)
		assert.NoError(t, err)
		err = cmd.RunE(cmd, cmd.Flags().Args())
		assert.NoError(t, err)
	})
}
