package main

import (
	"context"
	"os"
	"testing"

	"recac/internal/agent"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// KnowledgeTestMockAgent allows us to mock the Agent interface
type KnowledgeTestMockAgent struct {
	mock.Mock
}

func (m *KnowledgeTestMockAgent) Send(ctx context.Context, prompt string) (string, error) {
	args := m.Called(ctx, prompt)
	return args.String(0), args.Error(1)
}

func (m *KnowledgeTestMockAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	args := m.Called(ctx, prompt, onChunk)
	if onChunk != nil {
		onChunk(args.String(0))
	}
	return args.String(0), args.Error(1)
}

func TestKnowledgeAddAndList(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "recac-knowledge-test")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	cwd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(cwd)

	// 1. Test Add
	output, err := executeCommand(rootCmd, "knowledge", "add", "Rule 1: Be cool")
	assert.NoError(t, err)
	assert.Contains(t, output, "Added rule: Rule 1: Be cool")

	// Verify content
	content, err := os.ReadFile("AGENTS.md")
	assert.NoError(t, err)
	assert.Contains(t, string(content), "- Rule 1: Be cool")

	// 2. Test Add second rule (relies on auto-formatting)
	output, err = executeCommand(rootCmd, "knowledge", "add", "Rule 2: Stay cool")
	assert.NoError(t, err)
	assert.Contains(t, output, "Added rule: Rule 2: Stay cool")

	content, err = os.ReadFile("AGENTS.md")
	assert.NoError(t, err)
	assert.Contains(t, string(content), "- Rule 2: Stay cool")

	// 3. Test List
	output, err = executeCommand(rootCmd, "knowledge", "list")
	assert.NoError(t, err)
	assert.Contains(t, output, "Contents of AGENTS.md:")
	assert.Contains(t, output, "- Rule 1: Be cool")
	assert.Contains(t, output, "- Rule 2: Stay cool")
}

func TestKnowledgeCheck(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "recac-knowledge-check-test")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	cwd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(cwd)

	// Create AGENTS.md
	err = os.WriteFile("AGENTS.md", []byte("- No global variables\n"), 0644)
	assert.NoError(t, err)

	// Create a dummy file to check
	err = os.WriteFile("bad.go", []byte("var Global = 1"), 0644)
	assert.NoError(t, err)

	// Mock Agent
	mockAgent := new(KnowledgeTestMockAgent)
	originalAgentFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}
	defer func() { agentClientFactory = originalAgentFactory }()

	mockAgent.On("SendStream", mock.Anything, mock.Anything, mock.Anything).Return("Violation: Global variable detected", nil)

	// Run Check
	output, err := executeCommand(rootCmd, "knowledge", "check", "bad.go")
	assert.NoError(t, err)
	assert.Contains(t, output, "Violation: Global variable detected")

	// Verify prompt contained context
	mockAgent.AssertCalled(t, "SendStream", mock.Anything, mock.MatchedBy(func(prompt string) bool {
		return assert.Contains(t, prompt, "<project_rules>") &&
			   assert.Contains(t, prompt, "No global variables") &&
			   assert.Contains(t, prompt, "var Global = 1")
	}), mock.Anything)
}

func TestKnowledgeCheck_NoAgentsFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "recac-knowledge-fail-test")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	cwd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(cwd)

	output, err := executeCommand(rootCmd, "knowledge", "check", "foo.go")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "AGENTS.md not found")
	assert.Contains(t, output, "")
}
