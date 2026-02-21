package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"recac/internal/agent"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper to run typo command directly
func runTypoDirect(t *testing.T, args []string) string {
	b := new(bytes.Buffer)
	typoCmd.SetOut(b)
	typoCmd.SetErr(b)
	err := runTypo(typoCmd, args)
	require.NoError(t, err)
	return b.String()
}

func TestRunTypo_NoFiles(t *testing.T) {
	tmpDir := t.TempDir()

	origFactory := agentClientFactory
	defer func() { agentClientFactory = origFactory }()
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return agent.NewMockAgent(), nil
	}

	output := runTypoDirect(t, []string{tmpDir})
	assert.Contains(t, output, "No suitable files found to scan.")
}

func TestRunTypo_NoCandidates(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(filePath, []byte("func return package import"), 0644)

	origFactory := agentClientFactory
	defer func() { agentClientFactory = origFactory }()
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return agent.NewMockAgent(), nil
	}

	output := runTypoDirect(t, []string{tmpDir})
	assert.Contains(t, output, "No suspicious words found.")
}

func TestRunTypo_NoTypos(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(filePath, []byte("suspicious word"), 0644)

	origFactory := agentClientFactory
	defer func() { agentClientFactory = origFactory }()
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		mockAgent := agent.NewMockAgent()
		mockAgent.SetResponse("{}")
		return mockAgent, nil
	}

	output := runTypoDirect(t, []string{tmpDir})
	assert.Contains(t, output, "Found 2 unique candidate words")
	assert.Contains(t, output, "Checking with AI...")
	assert.Contains(t, output, "✅ No typos found!")
}

func TestRunTypo_WithTypos(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(filePath, []byte("This func has a typo: reciever"), 0644)

	origFactory := agentClientFactory
	defer func() { agentClientFactory = origFactory }()
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		mockAgent := agent.NewMockAgent()
		mockAgent.SetResponse(`{"reciever": "receiver"}`)
		return mockAgent, nil
	}

    // Ensure auto-fix is false
    oldVal := typoAutoFix
    typoAutoFix = false
    defer func() { typoAutoFix = oldVal }()

	output := runTypoDirect(t, []string{tmpDir})
	assert.Contains(t, output, "❌ Found 1 typos:")
	assert.Contains(t, output, "'reciever' -> 'receiver'")
	assert.Contains(t, output, "Run with --auto-fix")
}

func TestRunTypo_AutoFix(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(filePath, []byte("This func has a typo: reciever"), 0644)

	origFactory := agentClientFactory
	defer func() { agentClientFactory = origFactory }()
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		mockAgent := agent.NewMockAgent()
		mockAgent.SetResponse(`{"reciever": "receiver"}`)
		return mockAgent, nil
	}

	oldVal := typoAutoFix
	typoAutoFix = true
	defer func() { typoAutoFix = oldVal }()

	output := runTypoDirect(t, []string{tmpDir})

	assert.Contains(t, output, "✅ Fixed in")

	content, err := os.ReadFile(filePath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "receiver")
	assert.NotContains(t, string(content), "reciever")
}
