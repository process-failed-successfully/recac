package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/AlecAivazis/survey/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPersonaAdd_NonInteractive(t *testing.T) {
	// Setup temp dir for personas file
	tmpDir := t.TempDir()
	personasFile := filepath.Join(tmpDir, "personas.yaml")
	t.Setenv("RECAC_PERSONAS_FILE", personasFile)

	// Execute command
	output, err := executeCommand(rootCmd, "persona", "add", "mypersona",
		"--name", "My Persona",
		"--description", "Desc",
		"--prompt", "You are a bot")

	require.NoError(t, err)
	assert.Contains(t, output, "✅ Persona 'mypersona' saved")

	// Verify file content
	content, err := os.ReadFile(personasFile)
	require.NoError(t, err)
	assert.Contains(t, string(content), "My Persona")
	assert.Contains(t, string(content), "You are a bot")
}

func TestPersonaAdd_InteractiveFallback(t *testing.T) {
	// If flags are missing, it should trigger interactive mode.
	// We mock survey to provide answers.

	oldAsk := surveyAsk
	defer func() { surveyAsk = oldAsk }()

	surveyAsk = func(qs []*survey.Question, response interface{}, opts ...survey.AskOpt) error {
		// Fill response
		// response is a map[string]interface{} in our implementation
		// But in surveyAsk signature it is interface{}.
		// In implementation: answers := make(map[string]interface{})

		m := response.(*map[string]interface{})
		(*m)["id"] = "interactive-p"
		(*m)["name"] = "Interactive Persona"
		(*m)["description"] = "Interactive Desc"
		(*m)["system_prompt"] = "Interactive Prompt"
		return nil
	}

	tmpDir := t.TempDir()
	personasFile := filepath.Join(tmpDir, "personas.yaml")
	t.Setenv("RECAC_PERSONAS_FILE", personasFile)

	// Invoke without flags and args
	output, err := executeCommand(rootCmd, "persona", "add")
	require.NoError(t, err)
	assert.Contains(t, output, "✅ Persona 'interactive-p' saved")
}


func TestPersonaRemove_Force(t *testing.T) {
	// Setup temp dir for personas file
	tmpDir := t.TempDir()
	personasFile := filepath.Join(tmpDir, "personas.yaml")
	t.Setenv("RECAC_PERSONAS_FILE", personasFile)

	// Pre-create a persona file
	initialContent := `
mypersona:
  name: My Persona
  description: Desc
  system_prompt: You are a bot
`
	err := os.WriteFile(personasFile, []byte(initialContent), 0644)
	require.NoError(t, err)

	// Execute remove with force
	output, err := executeCommand(rootCmd, "persona", "remove", "mypersona", "--force")
	require.NoError(t, err)
	assert.Contains(t, output, "🗑️ Persona 'mypersona' removed")

	// Verify file content
	content, err := os.ReadFile(personasFile)
	require.NoError(t, err)
	assert.NotContains(t, string(content), "mypersona")
}

func TestPersonaRemove_NoForce_Aborted(t *testing.T) {
    // Save original and restore
    oldAskOne := surveyAskOneFunc
    defer func() { surveyAskOneFunc = oldAskOne }()

    // Mock to return false (Abort)
    surveyAskOneFunc = func(p survey.Prompt, response interface{}, opts ...survey.AskOpt) error {
        // Assume it's the confirm prompt
        // response is *bool
        *(response.(*bool)) = false
        return nil
    }

	// Setup temp dir
	tmpDir := t.TempDir()
	personasFile := filepath.Join(tmpDir, "personas.yaml")
	t.Setenv("RECAC_PERSONAS_FILE", personasFile)

    // Create dummy persona
    initialContent := `
mypersona:
  name: My Persona
  description: Desc
  system_prompt: You are a bot
`
    err := os.WriteFile(personasFile, []byte(initialContent), 0644)
    require.NoError(t, err)

    output, err := executeCommand(rootCmd, "persona", "remove", "mypersona")
    require.NoError(t, err)
    assert.Contains(t, output, "Aborted")

    // Verify NOT removed
    content, err := os.ReadFile(personasFile)
    require.NoError(t, err)
    assert.Contains(t, string(content), "mypersona")
}
