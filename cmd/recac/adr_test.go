package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"recac/internal/agent"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdrInit(t *testing.T) {
	// Setup temp dir and chdir
	tempDir := t.TempDir()
	origWd, _ := os.Getwd()
	os.Chdir(tempDir)
	defer os.Chdir(origWd)

	// Run init
	err := runAdrInit(adrInitCmd, []string{})
	require.NoError(t, err)

	// Verify
	adrDir := filepath.Join(tempDir, "docs", "adr")
	assert.DirExists(t, adrDir)
	assert.FileExists(t, filepath.Join(adrDir, "0000-use-adrs.md"))
}

func TestAdrNew(t *testing.T) {
	tempDir := t.TempDir()
	origWd, _ := os.Getwd()
	os.Chdir(tempDir)
	defer os.Chdir(origWd)

	// Init first
	runAdrInit(adrInitCmd, []string{})

	// Run new
	err := runAdrNew(adrNewCmd, []string{"My", "Decision"})
	require.NoError(t, err)

	// Verify file created
	// 0001-my-decision.md
	expectedFile := filepath.Join(tempDir, "docs", "adr", "0001-my-decision.md")
	assert.FileExists(t, expectedFile)

	content, _ := os.ReadFile(expectedFile)
	assert.Contains(t, string(content), "# 1. My Decision")
	assert.Contains(t, string(content), "Proposed") // Default status

	// Create another one
	runAdrNew(adrNewCmd, []string{"Another", "One"})
	expectedFile2 := filepath.Join(tempDir, "docs", "adr", "0002-another-one.md")
	assert.FileExists(t, expectedFile2)
}

func TestAdrList(t *testing.T) {
	tempDir := t.TempDir()
	origWd, _ := os.Getwd()
	os.Chdir(tempDir)
	defer os.Chdir(origWd)

	runAdrInit(adrInitCmd, []string{})
	runAdrNew(adrNewCmd, []string{"First"})

	// Capture stdout using bytes.Buffer
	var buf strings.Builder
	adrListCmd.SetOut(&buf)

	err := runAdrList(adrListCmd, []string{})
	adrListCmd.SetOut(nil) // Reset

	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "0000-use-adrs.md")
	assert.Contains(t, output, "0001-first.md")
}

func TestAdrGenerate(t *testing.T) {
	tempDir := t.TempDir()
	origWd, _ := os.Getwd()
	os.Chdir(tempDir)
	defer os.Chdir(origWd)

	runAdrInit(adrInitCmd, []string{})

	// Mock Agent
	origFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		m := agent.NewMockAgent()
		m.SetResponse("## Context\n\nAI generated context.\n\n## Decision\n\nAI decision.\n\n## Consequences\n\nAI consequences.")
		return m, nil
	}
	defer func() { agentClientFactory = origFactory }()

	// We need to simulate user input "y" to confirm saving
	// We can pipe stdin
	r, w, _ := os.Pipe()
	w.WriteString("y\n")
	w.Close()

	oldStdin := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = oldStdin }()

	err := runAdrGenerate(adrGenerateCmd, []string{"Switch", "to", "Postgres"})
	require.NoError(t, err)

	// Verify file created
	// Title was "Switch to Postgres" -> 0001-switch-to-postgres.md
	expectedFile := filepath.Join(tempDir, "docs", "adr", "0001-switch-to-postgres.md")
	assert.FileExists(t, expectedFile)

	content, _ := os.ReadFile(expectedFile)
	assert.Contains(t, string(content), "AI generated context")
}

func TestAdrInit_AlreadyExists(t *testing.T) {
	tempDir := t.TempDir()
	origWd, _ := os.Getwd()
	os.Chdir(tempDir)
	defer os.Chdir(origWd)

	// Create directory and file manually to simulate already initialized state
	adrDir := filepath.Join(tempDir, "docs", "adr")
	os.MkdirAll(adrDir, 0755)
	initFile := filepath.Join(adrDir, "0000-use-adrs.md")
	os.WriteFile(initFile, []byte("existing content"), 0644)

	err := runAdrInit(adrInitCmd, []string{})
	require.NoError(t, err)

	content, err := os.ReadFile(initFile)
	require.NoError(t, err)
	assert.Equal(t, "existing content", string(content))
}

func TestAdrNew_NoInit(t *testing.T) {
	tempDir := t.TempDir()
	origWd, _ := os.Getwd()
	os.Chdir(tempDir)
	defer os.Chdir(origWd)

	err := runAdrNew(adrNewCmd, []string{"My", "Decision"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ADR directory")
	assert.Contains(t, err.Error(), "does not exist")
}

func TestAdrList_NoInit(t *testing.T) {
	tempDir := t.TempDir()
	origWd, _ := os.Getwd()
	os.Chdir(tempDir)
	defer os.Chdir(origWd)

	err := runAdrList(adrListCmd, []string{})
	require.NoError(t, err) // It prints a message and returns nil if dir doesn't exist
}

func TestAdrGenerate_FactoryError(t *testing.T) {
	tempDir := t.TempDir()
	origWd, _ := os.Getwd()
	os.Chdir(tempDir)
	defer os.Chdir(origWd)

	// Mock Agent factory to return error
	origFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return nil, errors.New("factory failed")
	}
	defer func() { agentClientFactory = origFactory }()

	err := runAdrGenerate(adrGenerateCmd, []string{"Switch", "to", "Postgres"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "factory failed")
}

func TestAdrGenerate_Abort(t *testing.T) {
	tempDir := t.TempDir()
	origWd, _ := os.Getwd()
	os.Chdir(tempDir)
	defer os.Chdir(origWd)

	runAdrInit(adrInitCmd, []string{})

	origFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		m := agent.NewMockAgent()
		m.SetResponse("## Context\n\nAI generated context.")
		return m, nil
	}
	defer func() { agentClientFactory = origFactory }()

	r, w, _ := os.Pipe()
	w.WriteString("n\n") // Send 'n' to abort
	w.Close()

	oldStdin := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = oldStdin }()

	err := runAdrGenerate(adrGenerateCmd, []string{"Switch", "to", "Postgres"})
	require.NoError(t, err)

	expectedFile := filepath.Join(tempDir, "docs", "adr", "0001-switch-to-postgres.md")
	assert.NoFileExists(t, expectedFile)
}
