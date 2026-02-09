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
