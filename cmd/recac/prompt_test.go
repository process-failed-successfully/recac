package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPromptListCmd(t *testing.T) {
	// Setup buffer to capture output
	buf := new(bytes.Buffer)
	promptListCmd.SetOut(buf)

	err := promptListCmd.RunE(promptListCmd, []string{})
	assert.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "NAME")
	assert.Contains(t, output, "SOURCE")
	// We assume at least one standard prompt exists
	assert.Contains(t, output, "coding_agent")
}

func TestPromptOverrideAndResetCmd(t *testing.T) {
	// Save/Restore global flag
	oldGlobal := promptGlobal
	defer func() { promptGlobal = oldGlobal }()
	promptGlobal = false

	// 1. Setup Temp Dir as CWD
	tempDir := t.TempDir()
	originalWd, _ := os.Getwd()
	os.Chdir(tempDir)
	defer os.Chdir(originalWd)

	// 2. Override "coding_agent" (Local)
	buf := new(bytes.Buffer)
	promptOverrideCmd.SetOut(buf)

	err := promptOverrideCmd.RunE(promptOverrideCmd, []string{"coding_agent"})
	assert.NoError(t, err)

	// Check file existence
	expectedPath := filepath.Join(tempDir, ".recac", "prompts", "coding_agent.md")
	assert.FileExists(t, expectedPath)

	// 3. Verify Show reflects change (optional, depends on implementation details of GetPrompt)
	// We didn't change content, just copied it.

	// 4. Reset "coding_agent" (Local)
	buf.Reset()
	promptResetCmd.SetOut(buf)

	err = promptResetCmd.RunE(promptResetCmd, []string{"coding_agent"})
	assert.NoError(t, err)

	// Check file removed
	assert.NoFileExists(t, expectedPath)
}

func TestPromptGlobalOverride(t *testing.T) {
	// Save/Restore global flag
	oldGlobal := promptGlobal
	defer func() { promptGlobal = oldGlobal }()
	promptGlobal = true

	// 1. Setup Fake Home
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome) // For Unix
	t.Setenv("USERPROFILE", fakeHome) // For Windows

	// 2. Override "coding_agent" (Global)
	buf := new(bytes.Buffer)
	promptOverrideCmd.SetOut(buf)

	err := promptOverrideCmd.RunE(promptOverrideCmd, []string{"coding_agent"})
	assert.NoError(t, err)

	// Check file existence
	expectedPath := filepath.Join(fakeHome, ".recac", "prompts", "coding_agent.md")
	assert.FileExists(t, expectedPath)

	// 3. Reset "coding_agent" (Global)
	buf.Reset()
	promptResetCmd.SetOut(buf)

	err = promptResetCmd.RunE(promptResetCmd, []string{"coding_agent"})
	assert.NoError(t, err)

	// Check file removed
	assert.NoFileExists(t, expectedPath)
}
