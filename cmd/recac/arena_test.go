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
	// Save original flags
	origCompetitors := arenaCompetitors
	origTask := arenaTask
	origFile := arenaFile
	origJudgeProv := arenaJudgeProv
	origJudgeModel := arenaJudgeModel

	defer func() {
		agentClientFactory = originalFactory
		arenaCompetitors = origCompetitors
		arenaTask = origTask
		arenaFile = origFile
		arenaJudgeProv = origJudgeProv
		arenaJudgeModel = origJudgeModel
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

		// Set flags directly on variables since we might not be parsing args via cobra in unit test easily without resetting flags
		arenaCompetitors = "openai:gpt-4, gemini:gemini-pro"
		arenaTask = "Who are you?"
		arenaFile = ""
		arenaJudgeProv = "mock"
		arenaJudgeModel = "judge"

		err := runArena(arenaCmd, []string{})
		assert.NoError(t, err)
	})

	t.Run("Run Arena Validation Error", func(t *testing.T) {
		arenaCompetitors = "openai:gpt-4" // Only one
		err := runArena(arenaCmd, []string{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "requires at least 2 competitors")
	})

    t.Run("Run Arena With File Context", func(t *testing.T) {
        tmpDir := t.TempDir()
        tmpFile := tmpDir + "/context.txt"
        os.WriteFile(tmpFile, []byte("some context"), 0644)

        arenaCompetitors = "openai:gpt-4, gemini:gemini-pro"
        arenaTask = "Read file"
        arenaFile = tmpFile
        arenaJudgeProv = "mock"
        arenaJudgeModel = "judge"

        agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
             return &MockArenaAgent{Response: "OK"}, nil
        }

        err := runArena(arenaCmd, []string{})
        assert.NoError(t, err)
    })
}
