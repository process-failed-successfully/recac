package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"recac/internal/agent"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGlossaryCmd(t *testing.T) {
	// Setup temp dir with some code
	tmpDir := t.TempDir()
	err := os.WriteFile(filepath.Join(tmpDir, "domain.go"), []byte(`
package domain

// Session represents a user session.
type Session struct {
	ID string
}
`), 0644)
	require.NoError(t, err)

	// Mock Agent
	mockAg := agent.NewMockAgent()
	mockAg.SetResponse(`
[
  {
    "term": "Session",
    "definition": "Represents a user session.",
    "context": "domain.go"
  }
]
`)

	// Mock Factory
	originalFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAg, nil
	}
	defer func() { agentClientFactory = originalFactory }()

	// Execute
	output, err := executeCommand(rootCmd, "glossary", "--focus", tmpDir, "--limit", "10")
	require.NoError(t, err)

	assert.Contains(t, output, "Session")
	assert.Contains(t, output, "Represents a user session.")
	assert.Contains(t, output, "domain.go")
}

func TestGlossaryCmd_File(t *testing.T) {
	// Setup temp dir
	tmpDir := t.TempDir()
	codeFile := filepath.Join(tmpDir, "model.go")
	err := os.WriteFile(codeFile, []byte(`package model`), 0644)
	require.NoError(t, err)

	outFile := filepath.Join(tmpDir, "glossary.md")

	// Mock Agent
	mockAg := agent.NewMockAgent()
	mockAg.SetResponse(`[{"term": "Model", "definition": "A model.", "context": "model.go"}]`)

	// Mock Factory
	originalFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAg, nil
	}
	defer func() { agentClientFactory = originalFactory }()

	// Execute
	output, err := executeCommand(rootCmd, "glossary", "--focus", tmpDir, "--output", outFile)
	require.NoError(t, err)

	assert.Contains(t, output, "Glossary written to")

	content, err := os.ReadFile(outFile)
	require.NoError(t, err)
	assert.Contains(t, string(content), "| **Model** | A model. | `model.go` |")
}

func TestGlossaryCmd_NoTerms(t *testing.T) {
	// Setup temp dir
	tmpDir := t.TempDir()

	// Mock Agent
	mockAg := agent.NewMockAgent()
	mockAg.SetResponse(`[]`)

	// Mock Factory
	originalFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAg, nil
	}
	defer func() { agentClientFactory = originalFactory }()

	// Execute
	output, err := executeCommand(rootCmd, "glossary", "--focus", tmpDir)
	require.NoError(t, err)

	assert.Contains(t, output, "No domain terms found")
}
