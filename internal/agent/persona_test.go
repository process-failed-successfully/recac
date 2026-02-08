package agent

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPersonaManager_Defaults(t *testing.T) {
	pm := NewPersonaManager()

	// Check defaults exist
	p, ok := pm.GetPersona("default")
	assert.True(t, ok)
	assert.Equal(t, "Default", p.Name)

	p, ok = pm.GetPersona("security")
	assert.True(t, ok)
	assert.Equal(t, "Security Auditor", p.Name)
}

func TestPersonaManager_AddRemove(t *testing.T) {
	pm := NewPersonaManager()

	newP := Persona{
		Name: "Tester",
		Description: "A tester",
		SystemPrompt: "You are a tester.",
	}

	pm.AddPersona("test", newP)

	p, ok := pm.GetPersona("test")
	assert.True(t, ok)
	assert.Equal(t, "Tester", p.Name)

	// Remove custom
	err := pm.RemovePersona("test")
	assert.NoError(t, err)

	_, ok = pm.GetPersona("test")
	assert.False(t, ok)

	// Try remove default
	err = pm.RemovePersona("default")
	assert.Error(t, err)
}

func TestPersonaManager_SaveLoad(t *testing.T) {
	// Setup temp file
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "personas.yaml")
	os.Setenv("RECAC_PERSONAS_FILE", tmpFile)
	defer os.Unsetenv("RECAC_PERSONAS_FILE")

	// 1. Create manager, add persona, save
	pm1 := NewPersonaManager()
	pm1.AddPersona("custom1", Persona{Name: "Custom 1"})

	err := pm1.SavePersonas()
	require.NoError(t, err)

	// 2. Create new manager, load
	pm2 := NewPersonaManager()
	err = pm2.LoadPersonas()
	require.NoError(t, err)

	// Verify custom persona exists
	p, ok := pm2.GetPersona("custom1")
	assert.True(t, ok)
	assert.Equal(t, "Custom 1", p.Name)

	// Verify default exists
	_, ok = pm2.GetPersona("default")
	assert.True(t, ok)
}

func TestPersonaManager_OverrideDefault(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "personas.yaml")
	os.Setenv("RECAC_PERSONAS_FILE", tmpFile)
	defer os.Unsetenv("RECAC_PERSONAS_FILE")

	pm1 := NewPersonaManager()
	// Override default
	pm1.AddPersona("default", Persona{Name: "Overridden Default"})

	err := pm1.SavePersonas()
	require.NoError(t, err)

	pm2 := NewPersonaManager()
	err = pm2.LoadPersonas()
	require.NoError(t, err)

	p, ok := pm2.GetPersona("default")
	assert.True(t, ok)
	assert.Equal(t, "Overridden Default", p.Name)
}

func TestPersonaManager_Export_AllCustom(t *testing.T) {
	pm := NewPersonaManager()
	pm.AddPersona("custom1", Persona{Name: "Custom 1"})
	pm.AddPersona("custom2", Persona{Name: "Custom 2"})
	// Override default but same content (should not be exported)
	pm.AddPersona("default", DefaultPersonas["default"])

	data, err := pm.Export()
	require.NoError(t, err)

	exported, err := pm.Import(data) // Use Import to verify Export
	require.NoError(t, err)

	assert.Contains(t, exported, "custom1")
	assert.Contains(t, exported, "custom2")
	assert.NotContains(t, exported, "default") // Should not be exported as it matches default
}

func TestPersonaManager_Export_Specific(t *testing.T) {
	pm := NewPersonaManager()
	pm.AddPersona("custom1", Persona{Name: "Custom 1"})

	// Export existing
	data, err := pm.Export("custom1")
	require.NoError(t, err)

	exported, err := pm.Import(data)
	require.NoError(t, err)

	assert.Len(t, exported, 1)
	assert.Equal(t, "Custom 1", exported["custom1"].Name)

	// Export non-existent
	_, err = pm.Export("nonexistent")
	assert.Error(t, err)
}

func TestPersonaManager_Import_Valid(t *testing.T) {
	yamlData := []byte(`
newpersona:
  name: New Persona
  description: A new persona
  system_prompt: You are new.
`)

	pm := NewPersonaManager()
	imported, err := pm.Import(yamlData)
	require.NoError(t, err)

	assert.Contains(t, imported, "newpersona")
	assert.Equal(t, "New Persona", imported["newpersona"].Name)
}

func TestPersonaManager_Import_Invalid(t *testing.T) {
	yamlData := []byte(`invalid yaml`)

	pm := NewPersonaManager()
	_, err := pm.Import(yamlData)
	assert.Error(t, err)
}
