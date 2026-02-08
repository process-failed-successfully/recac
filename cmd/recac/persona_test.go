package main

import (
	"bytes"
	"os"
	"path/filepath"
	"recac/internal/agent"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPersonaListCmd(t *testing.T) {
	// Setup temp file for personas
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "personas.yaml")
	os.Setenv("RECAC_PERSONAS_FILE", tmpFile)
	defer os.Unsetenv("RECAC_PERSONAS_FILE")

	// Add a custom persona
	pm := agent.NewPersonaManager()
	pm.AddPersona("custom1", agent.Persona{Name: "Custom Persona", Description: "Desc", SystemPrompt: "Prompt"})
	require.NoError(t, pm.SavePersonas())

	cmd := personaListCmd
	var out bytes.Buffer
	cmd.SetOut(&out)

	err := cmd.RunE(cmd, []string{})
	require.NoError(t, err)

	output := out.String()
	assert.Contains(t, output, "custom1")
	assert.Contains(t, output, "Custom Persona")
	assert.Contains(t, output, "(custom)")
	assert.Contains(t, output, "default")
	assert.Contains(t, output, "(built-in)")
}

func TestPersonaShowCmd(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "personas.yaml")
	os.Setenv("RECAC_PERSONAS_FILE", tmpFile)
	defer os.Unsetenv("RECAC_PERSONAS_FILE")

	pm := agent.NewPersonaManager()
	pm.AddPersona("custom1", agent.Persona{Name: "Custom Persona", Description: "Desc", SystemPrompt: "Prompt"})
	require.NoError(t, pm.SavePersonas())

	cmd := personaShowCmd
	var out bytes.Buffer
	cmd.SetOut(&out)

	// Test existing custom persona
	err := cmd.RunE(cmd, []string{"custom1"})
	require.NoError(t, err)
	output := out.String()
	assert.Contains(t, output, "Name:        Custom Persona")
	assert.Contains(t, output, "ID:          custom1")
	assert.Contains(t, output, "Description: Desc")
	assert.Contains(t, output, "Prompt")

	// Test existing built-in persona
	out.Reset()
	err = cmd.RunE(cmd, []string{"default"})
	require.NoError(t, err)
	output = out.String()
	assert.Contains(t, output, "Name:        Default")

	// Test non-existent persona
	err = cmd.RunE(cmd, []string{"nonexistent"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestPersonaExportCmd(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "personas.yaml")
	os.Setenv("RECAC_PERSONAS_FILE", tmpFile)
	defer os.Unsetenv("RECAC_PERSONAS_FILE")

	pm := agent.NewPersonaManager()
	pm.AddPersona("custom1", agent.Persona{Name: "Custom1", Description: "Desc", SystemPrompt: "Prompt"})
	require.NoError(t, pm.SavePersonas())

	// Test export all
	cmd := personaExportCmd
	var out bytes.Buffer
	cmd.SetOut(&out)

	err := cmd.RunE(cmd, []string{})
	require.NoError(t, err)
	output := out.String()
	assert.Contains(t, output, "custom1")
	assert.Contains(t, output, "Custom1")
	assert.NotContains(t, output, "default") // Default is built-in, so not exported by default

	// Test export single (custom)
	out.Reset()
	err = cmd.RunE(cmd, []string{"custom1"})
	require.NoError(t, err)
	output = out.String()
	assert.Contains(t, output, "custom1:")
	assert.Contains(t, output, "name: Custom1")

	// Test export single (built-in)
	out.Reset()
	err = cmd.RunE(cmd, []string{"default"})
	require.NoError(t, err)
	output = out.String()
	assert.Contains(t, output, "default:")
	assert.Contains(t, output, "name: Default")
}

func TestPersonaImportCmd(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "personas.yaml")
	os.Setenv("RECAC_PERSONAS_FILE", tmpFile)
	defer os.Unsetenv("RECAC_PERSONAS_FILE")

	// Create import file
	importFile := filepath.Join(tmpDir, "import.yaml")
	content := `
imported1:
  name: Imported Persona
  description: Imported Desc
  system_prompt: Imported Prompt
`
	require.NoError(t, os.WriteFile(importFile, []byte(content), 0644))

	cmd := personaImportCmd
	var out bytes.Buffer
	cmd.SetOut(&out)

	err := cmd.RunE(cmd, []string{importFile})
	require.NoError(t, err)
	assert.Contains(t, out.String(), "Successfully imported 1 personas")

	// Verify imported
	pm := agent.NewPersonaManager()
	require.NoError(t, pm.LoadPersonas())
	p, ok := pm.GetPersona("imported1")
	assert.True(t, ok)
	assert.Equal(t, "Imported Persona", p.Name)
}

func TestPersonaRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "personas.yaml")
	os.Setenv("RECAC_PERSONAS_FILE", tmpFile)
	defer os.Unsetenv("RECAC_PERSONAS_FILE")

	// 1. Add custom persona
	pm := agent.NewPersonaManager()
	pm.AddPersona("trip", agent.Persona{Name: "Trip", Description: "Desc", SystemPrompt: "Prompt"})
	require.NoError(t, pm.SavePersonas())

	// 2. Export it
	exportCmd := personaExportCmd
	var exportOut bytes.Buffer
	exportCmd.SetOut(&exportOut)
	err := exportCmd.RunE(exportCmd, []string{"trip"})
	require.NoError(t, err)

	// 3. Remove it
	removeCmd := personaRemoveCmd
	removeCmd.SetOut(&bytes.Buffer{}) // Quiet
	err = removeCmd.RunE(removeCmd, []string{"trip"})
	require.NoError(t, err)

	pm = agent.NewPersonaManager()
	require.NoError(t, pm.LoadPersonas())
	_, ok := pm.GetPersona("trip")
	assert.False(t, ok, "Persona should be removed")

	// 4. Import it back (via stdin)
	importCmd := personaImportCmd
	var importOut bytes.Buffer
	importCmd.SetOut(&importOut)
	importCmd.SetIn(&exportOut) // Pipe export output to import input

	err = importCmd.RunE(importCmd, []string{"-"})
	require.NoError(t, err)

	// 5. Verify it's back
	pm = agent.NewPersonaManager()
	require.NoError(t, pm.LoadPersonas())
	p, ok := pm.GetPersona("trip")
	assert.True(t, ok)
	assert.Equal(t, "Trip", p.Name)
}
