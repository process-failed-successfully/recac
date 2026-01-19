package main

import (
	"context"
	"fmt"
	"os"
	"testing"

	"recac/internal/agent"

	"github.com/AlecAivazis/survey/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// SuggestTestMockAgent allows us to mock the Agent interface
type SuggestTestMockAgent struct {
	mock.Mock
}

func (m *SuggestTestMockAgent) Send(ctx context.Context, prompt string) (string, error) {
	args := m.Called(ctx, prompt)
	return args.String(0), args.Error(1)
}

func (m *SuggestTestMockAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	args := m.Called(ctx, prompt, onChunk)
	return args.String(0), args.Error(1)
}

func TestSuggestCmd(t *testing.T) {
	// Setup temporary directory for TODO.md
	tmpDir, err := os.MkdirTemp("", "recac-suggest-test")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	cwd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(cwd)

	// Mock Agent
	mockAgent := new(SuggestTestMockAgent)
	originalAgentFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}
	defer func() { agentClientFactory = originalAgentFactory }()

	// Mock Survey
	originalAskOne := askOneFunc
	// We need to simulate user answers. Since the loop runs twice, we need to handle sequential calls.
	callCount := 0
	askOneFunc = func(p survey.Prompt, response interface{}, opts ...survey.AskOpt) error {
		callCount++
		// assert prompt type
		if _, ok := p.(*survey.Select); ok {
			// Set response to "Add to TODO"
			// response is a pointer to string
			*(response.(*string)) = "Add to TODO"
			return nil
		}
		return fmt.Errorf("unexpected prompt type")
	}
	defer func() { askOneFunc = originalAskOne }()

	// Test Data
	jsonResponse := `[
		{
			"title": "Fix nil pointer",
			"description": "Check for nil before accessing field",
			"type": "bug",
			"file": "main.go"
		},
		{
			"title": "Refactor huge function",
			"description": "Split runSuggest into smaller functions",
			"type": "refactor"
		}
	]`

	mockAgent.On("Send", mock.Anything, mock.Anything).Return(jsonResponse, nil)

	// Execute
	// We use suggestCmd directly or via executeCommand helper
	// Note: executeCommand uses a fresh root command clone logic or resets flags?
	// The helper `executeCommand` in test_helpers_test.go uses global `rootCmd`.
	// Since `suggestCmd` is added to `rootCmd` in init(), it should work.

	output, err := executeCommand(rootCmd, "suggest")
	assert.NoError(t, err)

	// Verify Output
	assert.Contains(t, output, "Found 2 suggestions")
	assert.Contains(t, output, "Fix nil pointer")
	assert.Contains(t, output, "Refactor huge function")

	// Verify TODO.md content
	content, err := os.ReadFile("TODO.md")
	assert.NoError(t, err)
	todoContent := string(content)
	assert.Contains(t, todoContent, "Fix nil pointer (bug)")
	assert.Contains(t, todoContent, "Refactor huge function (refactor)")
}

func TestSuggestCmd_NoSuggestions(t *testing.T) {
	// Setup temporary directory
	tmpDir, err := os.MkdirTemp("", "recac-suggest-test-empty")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	cwd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(cwd)

	// Mock Agent
	mockAgent := new(SuggestTestMockAgent)
	originalAgentFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}
	defer func() { agentClientFactory = originalAgentFactory }()

	mockAgent.On("Send", mock.Anything, mock.Anything).Return("[]", nil)

	output, err := executeCommand(rootCmd, "suggest")
	assert.NoError(t, err)

	assert.Contains(t, output, "No suggestions found")
}

func TestSuggestCmd_AgentFailure(t *testing.T) {
	// Setup temporary directory
	tmpDir, err := os.MkdirTemp("", "recac-suggest-test-fail")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	cwd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(cwd)

	// Mock Agent
	mockAgent := new(SuggestTestMockAgent)
	originalAgentFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}
	defer func() { agentClientFactory = originalAgentFactory }()

	mockAgent.On("Send", mock.Anything, mock.Anything).Return("", fmt.Errorf("agent error"))

	_, err = executeCommand(rootCmd, "suggest")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "agent failed")
}
