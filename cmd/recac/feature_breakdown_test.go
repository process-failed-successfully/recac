package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"recac/internal/agent"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// BreakdownMockAgent satisfies the agent.Agent interface for testing.
type BreakdownMockAgent struct {
	response string
}

func (m *BreakdownMockAgent) Send(ctx context.Context, prompt string) (string, error) {
	return m.response, nil
}
func (m *BreakdownMockAgent) SendWithImage(ctx context.Context, prompt string, imagePath string) (string, error) {
	return "", nil
}
func (m *BreakdownMockAgent) SendStream(ctx context.Context, prompt string, callback func(string)) (string, error) {
	callback(m.response)
	return m.response, nil
}

func TestFeatureBreakdownCmd(t *testing.T) {
	// Setup temporary workspace
	tmpDir, err := os.MkdirTemp("", "recac-feature-breakdown-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Change working directory to temp dir
	cwd, _ := os.Getwd()
	defer os.Chdir(cwd)
	os.Chdir(tmpDir)

	// Override factory to use mock agent
	mockResponse := "```json\n" + `["Task 1", "Task 2"]` + "\n```"
	mockAgent := &BreakdownMockAgent{response: mockResponse}

	origFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}
	defer func() { agentClientFactory = origFactory }()

	// Execute command via root to respect nested arguments
	rootCmd.SetArgs([]string{"feature", "breakdown", "test feature"})

	// Redirect stdout to avoid clutter
	rootCmd.SetOut(os.Stdout)
	rootCmd.SetErr(os.Stderr)

	err = rootCmd.Execute()
	require.NoError(t, err)

	// Verify TODO.md was updated
	todoPath := filepath.Join(tmpDir, "TODO.md")
	require.FileExists(t, todoPath)

	content, err := os.ReadFile(todoPath)
	require.NoError(t, err)

	strContent := string(content)
	assert.Contains(t, strContent, "# TODO")
	assert.Contains(t, strContent, "- [ ] Task 1")
	assert.Contains(t, strContent, "- [ ] Task 2")
}

func TestFeatureBreakdownCmd_EmptyResponse(t *testing.T) {
	// Setup temporary workspace
	tmpDir, err := os.MkdirTemp("", "recac-feature-breakdown-test-empty")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Change working directory to temp dir
	cwd, _ := os.Getwd()
	defer os.Chdir(cwd)
	os.Chdir(tmpDir)

	// Override factory to use mock agent with empty JSON array
	mockResponse := "```json\n[]\n```"
	mockAgent := &BreakdownMockAgent{response: mockResponse}

	origFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}
	defer func() { agentClientFactory = origFactory }()

	// Execute command
	rootCmd.SetArgs([]string{"feature", "breakdown", "test feature"})
	err = rootCmd.Execute()
	require.NoError(t, err)

	// Verify TODO.md was NOT created or is empty since no tasks were added
	todoPath := filepath.Join(tmpDir, "TODO.md")
	_, err = os.Stat(todoPath)
	assert.True(t, os.IsNotExist(err), "TODO.md should not be created if no tasks were generated")
}

func TestFeatureBreakdownCmd_ExistingTODO(t *testing.T) {
	// Setup temporary workspace
	tmpDir, err := os.MkdirTemp("", "recac-feature-breakdown-test-existing")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create an existing TODO.md
	todoPath := filepath.Join(tmpDir, "TODO.md")
	initialContent := "# TODO\n\n- [ ] Initial Task\n"
	err = os.WriteFile(todoPath, []byte(initialContent), 0644)
	require.NoError(t, err)

	// Change working directory to temp dir
	cwd, _ := os.Getwd()
	defer os.Chdir(cwd)
	os.Chdir(tmpDir)

	// Override factory to use mock agent
	mockResponse := "```json\n" + `["New Task"]` + "\n```"
	mockAgent := &BreakdownMockAgent{response: mockResponse}

	origFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}
	defer func() { agentClientFactory = origFactory }()

	// Execute command
	rootCmd.SetArgs([]string{"feature", "breakdown", "test feature"})
	err = rootCmd.Execute()
	require.NoError(t, err)

	// Verify TODO.md was updated
	content, err := os.ReadFile(todoPath)
	require.NoError(t, err)

	strContent := string(content)
	// It should only have one "# TODO"
	assert.Equal(t, 1, strings.Count(strContent, "# TODO"), "Should only have one header")
	assert.Contains(t, strContent, "- [ ] Initial Task")
	assert.Contains(t, strContent, "- [ ] New Task")
}
