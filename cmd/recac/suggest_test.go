package main

import (
	"bytes"
	"context"
	"testing"

	"recac/internal/agent"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

// MockSuggestAgent implements agent.Agent for testing
type MockSuggestAgent struct {
	Response string
}

func (m *MockSuggestAgent) Send(ctx context.Context, prompt string) (string, error) {
	return m.Response, nil
}

func (m *MockSuggestAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	return m.Send(ctx, prompt)
}

func TestRunSuggest(t *testing.T) {
	// 1. Mock Context Generator
	origContextFunc := generateContextFunc
	generateContextFunc = func(opts ContextOptions) (string, error) {
		return "Mock Codebase Context", nil
	}
	defer func() { generateContextFunc = origContextFunc }()

	// 2. Mock Agent Factory
	origAgentFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return &MockSuggestAgent{
			Response: `[
				{
					"title": "Fix Typo",
					"description": "There is a typo in main.go",
					"type": "refactor",
					"file": "main.go"
				},
				{
					"title": "Add Logging",
					"description": "Add logging to server.go",
					"type": "feature"
				}
			]`,
		}, nil
	}
	defer func() { agentClientFactory = origAgentFactory }()

	// 3. Prepare Command
	cmd := &cobra.Command{}
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	// 4. Run Suggest
	err := runSuggest(cmd, []string{})
	assert.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "Found 2 suggestions")
	assert.Contains(t, output, "Fix Typo (REFACTOR)")
	assert.Contains(t, output, "Add Logging (FEATURE)")
}

func TestRunSuggest_NoSuggestions(t *testing.T) {
	// 1. Mock Context Generator
	origContextFunc := generateContextFunc
	generateContextFunc = func(opts ContextOptions) (string, error) {
		return "Mock Codebase Context", nil
	}
	defer func() { generateContextFunc = origContextFunc }()

	// 2. Mock Agent Factory (Empty List)
	origAgentFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return &MockSuggestAgent{
			Response: `[]`,
		}, nil
	}
	defer func() { agentClientFactory = origAgentFactory }()

	// 3. Prepare Command
	cmd := &cobra.Command{}
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	// 4. Run Suggest
	err := runSuggest(cmd, []string{})
	assert.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "No suggestions found")
}

func TestRunSuggest_InvalidJSON(t *testing.T) {
	// 1. Mock Context Generator
	origContextFunc := generateContextFunc
	generateContextFunc = func(opts ContextOptions) (string, error) {
		return "Mock Codebase Context", nil
	}
	defer func() { generateContextFunc = origContextFunc }()

	// 2. Mock Agent Factory (Invalid JSON)
	origAgentFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return &MockSuggestAgent{
			Response: `Not JSON`,
		}, nil
	}
	defer func() { agentClientFactory = origAgentFactory }()

	// 3. Prepare Command
	cmd := &cobra.Command{}
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	// 4. Run Suggest
	err := runSuggest(cmd, []string{})
	assert.NoError(t, err) // Should not error, just print warning

	output := buf.String()
	assert.Contains(t, output, "Failed to parse JSON response")
}

func TestRunSuggest_ContextError(t *testing.T) {
	// 1. Mock Context Generator (Error)
	origContextFunc := generateContextFunc
	generateContextFunc = func(opts ContextOptions) (string, error) {
		return "", assert.AnError
	}
	defer func() { generateContextFunc = origContextFunc }()

	// 3. Prepare Command
	cmd := &cobra.Command{}
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	// 4. Run Suggest
	err := runSuggest(cmd, []string{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to generate codebase context")
}
