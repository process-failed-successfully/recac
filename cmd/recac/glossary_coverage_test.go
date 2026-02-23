package main

import (
	"context"
	"errors"
	"os"
	"testing"

	"recac/internal/agent"

	"github.com/stretchr/testify/assert"
)

func TestGlossaryCmd_AgentInitError(t *testing.T) {
	// Setup
	originalFactory := agentClientFactory
	defer func() { agentClientFactory = originalFactory }()

	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return nil, errors.New("agent init failed")
	}

	// Execute
	_, err := executeCommand(rootCmd, "glossary")

	// Verify
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to initialize agent")
}

func TestGlossaryCmd_AgentSendError(t *testing.T) {
	// Setup
	originalFactory := agentClientFactory
	defer func() { agentClientFactory = originalFactory }()

	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return &MockErrorAgent{Err: errors.New("send failed")}, nil
	}

	// Execute
	_, err := executeCommand(rootCmd, "glossary")

	// Verify
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "agent failed to generate glossary")
}

func TestGlossaryCmd_JSONError(t *testing.T) {
	// Setup
	originalFactory := agentClientFactory
	defer func() { agentClientFactory = originalFactory }()

	mockAg := agent.NewMockAgent()
	mockAg.SetResponse("invalid json")

	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAg, nil
	}

	// Execute
	output, err := executeCommand(rootCmd, "glossary")

	// Verify
	assert.NoError(t, err) // It logs warning and returns nil
	assert.Contains(t, output, "Warning: Failed to parse JSON response")
}

func TestGlossaryCmd_WriteFileError(t *testing.T) {
	// Setup
	tmpDir := t.TempDir()
	outFile := "glossary.md" // Relative path, resolved to CWD

	// Mock Agent
	mockAg := agent.NewMockAgent()
	mockAg.SetResponse(`[{"term": "Test", "definition": "Test def", "context": "test.go"}]`)

	originalFactory := agentClientFactory
	defer func() { agentClientFactory = originalFactory }()

	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAg, nil
	}

	// Mock writeFileFunc
	originalWriteFile := writeFileFunc
	defer func() { writeFileFunc = originalWriteFile }()

	writeFileFunc = func(name string, data []byte, perm os.FileMode) error {
		return errors.New("write failed")
	}

	// We need to change CWD to temp dir so output file is written there (or check logic)
	cwd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(cwd)

	// Execute
	_, err := executeCommand(rootCmd, "glossary", "--output", outFile)

	// Verify
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to write output file")
}
