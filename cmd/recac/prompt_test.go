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

func TestPromptHasOverrides(t *testing.T) {
	// 1. Env override
	tempDirEnv := t.TempDir()
	t.Setenv("RECAC_PROMPTS_DIR", tempDirEnv)

	// Should be false initially
	assert.False(t, hasEnvOverride("test_prompt"))

	// Create file
	err := os.WriteFile(filepath.Join(tempDirEnv, "test_prompt.md"), []byte("env override"), 0644)
	assert.NoError(t, err)
	assert.True(t, hasEnvOverride("test_prompt"))

	// 2. Local override
	tempDirLocal := t.TempDir()
	originalWd, _ := os.Getwd()
	os.Chdir(tempDirLocal)
	defer os.Chdir(originalWd)

	assert.False(t, hasLocalOverride("test_prompt"))

	promptsDir := filepath.Join(tempDirLocal, ".recac", "prompts")
	err = os.MkdirAll(promptsDir, 0755)
	assert.NoError(t, err)
	err = os.WriteFile(filepath.Join(promptsDir, "test_prompt.md"), []byte("local override"), 0644)
	assert.NoError(t, err)
	assert.True(t, hasLocalOverride("test_prompt"))

	// 3. Global override
	tempDirGlobal := t.TempDir()
	t.Setenv("HOME", tempDirGlobal) // For Unix
	t.Setenv("USERPROFILE", tempDirGlobal) // For Windows

	assert.False(t, hasGlobalOverride("test_prompt"))

	promptsDirGlobal := filepath.Join(tempDirGlobal, ".recac", "prompts")
	err = os.MkdirAll(promptsDirGlobal, 0755)
	assert.NoError(t, err)
	err = os.WriteFile(filepath.Join(promptsDirGlobal, "test_prompt.md"), []byte("global override"), 0644)
	assert.NoError(t, err)
	assert.True(t, hasGlobalOverride("test_prompt"))
}

func TestPromptShowCmd(t *testing.T) {
	buf := new(bytes.Buffer)
	promptShowCmd.SetOut(buf)

	err := promptShowCmd.RunE(promptShowCmd, []string{"coding_agent"})
	assert.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "YOUR ROLE - CODING AGENT")
}

func TestPromptShowCmd_Error(t *testing.T) {
	buf := new(bytes.Buffer)
	promptShowCmd.SetOut(buf)

	err := promptShowCmd.RunE(promptShowCmd, []string{"non_existent_prompt"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "file does not exist")
}

func TestPromptResetCmd_Error(t *testing.T) {
	buf := new(bytes.Buffer)
	promptResetCmd.SetOut(buf)

	err := promptResetCmd.RunE(promptResetCmd, []string{"non_existent_prompt_to_reset"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "override not found at")
}
