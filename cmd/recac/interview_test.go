package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"recac/internal/agent"
	"testing"

	"github.com/AlecAivazis/survey/v2"
	"github.com/stretchr/testify/assert"
)

// MockAgentForInterview is a mock agent that returns responses from a queue
type MockAgentForInterview struct {
	Responses []string
	Index     int
}

func (m *MockAgentForInterview) Send(ctx context.Context, prompt string) (string, error) {
	if m.Index >= len(m.Responses) {
		return "", fmt.Errorf("no more responses")
	}
	resp := m.Responses[m.Index]
	m.Index++
	return resp, nil
}

func (m *MockAgentForInterview) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	return m.Send(ctx, prompt)
}

func TestInterviewCmd(t *testing.T) {
	// Setup overrides
	originalAskOne := askOneFunc
	originalAgentFactory := agentClientFactory
	defer func() {
		askOneFunc = originalAskOne
		agentClientFactory = originalAgentFactory
	}()

	t.Run("Success Flow with Prompt", func(t *testing.T) {
		// Mock User Inputs
		// 1. "A todo app" (Initial topic)
		// 2. "Go and React" (Answer to Q1)
		inputs := []string{
			"A todo app",
			"Go and React",
		}
		inputIndex := 0

		askOneFunc = func(p survey.Prompt, response interface{}, opts ...survey.AskOpt) error {
			if input, ok := p.(*survey.Input); ok {
				// Verify we have inputs left
				if inputIndex >= len(inputs) {
					return fmt.Errorf("unexpected input prompt: %s", input.Message)
				}
				if strPtr, ok := response.(*string); ok {
					*strPtr = inputs[inputIndex]
					inputIndex++
					return nil
				}
			}
			return fmt.Errorf("unexpected prompt type: %T", p)
		}

		// Mock Agent Responses
		mockAgent := &MockAgentForInterview{
			Responses: []string{
				"What tech stack?", // Q1
				"'''spec\nTitle: Todo App\nStack: Go/React\n'''\nSPEC_COMPLETE", // Final
			},
		}

		agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
			return mockAgent, nil
		}

		// Run Command Logic Directly
		tmpDir := t.TempDir()
		outputFile := filepath.Join(tmpDir, "spec.txt")

		// Manually set output variable since we bypass flag parsing
		interviewOutput = outputFile

		// We pass empty args, so it asks "What would you like to build?"
		err := runInterview(interviewCmd, []string{})
		assert.NoError(t, err)

		// Verify Output
		content, err := os.ReadFile(outputFile)
		assert.NoError(t, err)
		assert.Contains(t, string(content), "Title: Todo App")
		assert.Contains(t, string(content), "Stack: Go/React")
	})

	t.Run("Success Flow with Args", func(t *testing.T) {
		// User provides topic in args, so only one input needed (answer to Q1)
		inputs := []string{
			"Go and React",
		}
		inputIndex := 0

		askOneFunc = func(p survey.Prompt, response interface{}, opts ...survey.AskOpt) error {
			if _, ok := p.(*survey.Input); ok {
				if strPtr, ok := response.(*string); ok {
					*strPtr = inputs[inputIndex]
					inputIndex++
					return nil
				}
			}
			return fmt.Errorf("unexpected prompt type: %T", p)
		}

		mockAgent := &MockAgentForInterview{
			Responses: []string{
				"What tech stack?",
				"'''spec\nTitle: Chat App\nStack: Go/React\n'''\nSPEC_COMPLETE",
			},
		}

		agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
			return mockAgent, nil
		}

		tmpDir := t.TempDir()
		outputFile := filepath.Join(tmpDir, "chat_spec.txt")

		interviewOutput = outputFile
		// Pass args directly
		err := runInterview(interviewCmd, []string{"A chat app"})
		assert.NoError(t, err)

		content, err := os.ReadFile(outputFile)
		assert.NoError(t, err)
		assert.Contains(t, string(content), "Title: Chat App")
	})

	t.Run("Overwrite Check", func(t *testing.T) {
		tmpDir := t.TempDir()
		outputFile := filepath.Join(tmpDir, "existing.txt")
		os.WriteFile(outputFile, []byte("old"), 0644)

		askOneFunc = func(p survey.Prompt, response interface{}, opts ...survey.AskOpt) error {
			if confirm, ok := p.(*survey.Confirm); ok {
				assert.Contains(t, confirm.Message, "Overwrite?")
				if boolPtr, ok := response.(*bool); ok {
					*boolPtr = false // Do not overwrite
					return nil
				}
			}
			return fmt.Errorf("unexpected prompt type")
		}

		interviewOutput = outputFile
		err := runInterview(interviewCmd, []string{"Topic"})
		assert.NoError(t, err) // Should succeed but do nothing

		// Content should be unchanged
		content, _ := os.ReadFile(outputFile)
		assert.Equal(t, "old", string(content))
	})
}
