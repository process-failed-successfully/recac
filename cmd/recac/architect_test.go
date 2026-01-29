package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"recac/internal/agent"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)


func TestRunArchitectCmd(t *testing.T) {
	// Setup temporary directory
	tempDir := t.TempDir()
	specPath := filepath.Join(tempDir, "app_spec.txt")
	outDir := filepath.Join(tempDir, "out")

	err := os.WriteFile(specPath, []byte("Test App Spec"), 0644)
	require.NoError(t, err)

	// Mock Agent Response
	mockResponse := map[string]string{
		"architecture.yaml": `version: "1.0"
system_name: TestApp
components:
  - id: api
    name: API
    type: service
    description: API Service
contracts: []
`,
		"contracts/api.yaml": "openapi: 3.0.0\ninfo:\n  title: API\n  version: 1.0.0",
	}
	jsonBytes, _ := json.Marshal(mockResponse)
	jsonStr := "```json\n" + string(jsonBytes) + "\n```"

	mockAgent := new(MockAgent)
	mockAgent.On("Send", mock.Anything, mock.Anything).Return(jsonStr, nil)

	// Mock agentClientFactory
	originalFactory := agentClientFactory
	defer func() { agentClientFactory = originalFactory }()
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}

	// Mock exit to prevent test termination
	originalExit := exit
	defer func() { exit = originalExit }()
	var exitCode int
	exit = func(code int) {
		exitCode = code
	}

	// Set flags
	cmd := architectCmd
	cmd.Flags().Set("spec", specPath)
	cmd.Flags().Set("out", outDir)

	// Mock Viper
	viper.Set("provider", "mock")
	viper.Set("model", "mock-model")

	// Capture stdout/stderr? Using a pipe or simple execution.
	// Since we mock the agent and file ops, we can just run it.
	// Note: runArchitectCmd prints to stdout/stderr.

	// Execute
	runArchitectCmd(cmd, []string{})

	// Verify
	assert.Equal(t, 0, exitCode, "Expected exit code 0 (success)")

	// Verify files created
	require.FileExists(t, filepath.Join(outDir, "architecture.yaml"))
	require.FileExists(t, filepath.Join(outDir, "contracts/api.yaml"))

	content, err := os.ReadFile(filepath.Join(outDir, "architecture.yaml"))
	require.NoError(t, err)
	assert.Contains(t, string(content), "name: TestApp")
}
