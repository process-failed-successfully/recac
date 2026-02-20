package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"recac/internal/agent"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTodoEstimate(t *testing.T) {
	// Create a temporary directory
	tmpDir, err := os.MkdirTemp("", "recac-todo-estimate-test-")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Switch to tmpDir
	cwd, err := os.Getwd()
	require.NoError(t, err)
	defer os.Chdir(cwd)
	require.NoError(t, os.Chdir(tmpDir))

	// Create a dummy file
	dummyFile := "internal/test.go"
	require.NoError(t, os.MkdirAll(filepath.Dir(dummyFile), 0755))
	err = os.WriteFile(dummyFile, []byte("package test\n\n// TODO: Fix this bug\nfunc Test() {}\n"), 0644)
	require.NoError(t, err)

	// Create TODO.md
	todoContent := "- [ ] [internal/test.go:3] TODO: Fix this bug\n"
	err = os.WriteFile("TODO.md", []byte(todoContent), 0644)
	require.NoError(t, err)

	// Mock Agent
	mockAgent := agent.NewMockAgent()
	mockResponse := `{
  "summary": "Fix the bug by adding error handling.",
  "complexity": "Low",
  "story_points": 1,
  "estimated_hours": "1h",
  "risks": [],
  "implementation_steps": ["Add error check"]
}`
	mockAgent.SetResponse(mockResponse)

	// Override factory
	originalFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}
	defer func() { agentClientFactory = originalFactory }()

	// Execute command
	output, err := executeCommand(rootCmd, "todo", "estimate", "1")
	if err != nil {
		fmt.Printf("Command Output on Error: %s\n", output)
	}
	require.NoError(t, err)

	// Verify output
	assert.Contains(t, output, "Estimating TODO in internal/test.go at line 3")
	assert.Contains(t, output, "Fix the bug by adding error handling")
	assert.Contains(t, output, "Complexity")
	assert.Contains(t, output, "Low")
	assert.Contains(t, output, "Story Points")
	assert.Contains(t, output, "1")
}

func TestTodoEstimate_InvalidIndex(t *testing.T) {
	// Create a temporary directory
	tmpDir, err := os.MkdirTemp("", "recac-todo-estimate-test-invalid-")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Switch to tmpDir
	cwd, err := os.Getwd()
	require.NoError(t, err)
	defer os.Chdir(cwd)
	require.NoError(t, os.Chdir(tmpDir))

	// Create TODO.md
	todoContent := "- [ ] [internal/test.go:3] TODO: Fix this bug\n"
	err = os.WriteFile("TODO.md", []byte(todoContent), 0644)
	require.NoError(t, err)

	// Execute command with invalid index
	_, err = executeCommand(rootCmd, "todo", "estimate", "99")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "task index 99 not found")
}

func TestTodoEstimate_FileNotFound(t *testing.T) {
	// Create a temporary directory
	tmpDir, err := os.MkdirTemp("", "recac-todo-estimate-test-nofile-")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Switch to tmpDir
	cwd, err := os.Getwd()
	require.NoError(t, err)
	defer os.Chdir(cwd)
	require.NoError(t, os.Chdir(tmpDir))

	// Create TODO.md pointing to non-existent file
	todoContent := "- [ ] [nonexistent.go:1] TODO: Fix this bug\n"
	err = os.WriteFile("TODO.md", []byte(todoContent), 0644)
	require.NoError(t, err)

	// Execute command
	_, err = executeCommand(rootCmd, "todo", "estimate", "1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read target file nonexistent.go")
}
