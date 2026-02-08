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

	// Remove default (reset)
	err = pm.RemovePersona("default")
	assert.NoError(t, err) // Should succeed now (resetting to default)

	p, ok = pm.GetPersona("default")
	assert.True(t, ok)
	assert.Equal(t, "Default", p.Name) // Still there
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

	// Now remove (reset)
	err = pm2.RemovePersona("default")
	require.NoError(t, err)

	// Should be back to original default in memory
	p, ok = pm2.GetPersona("default")
	assert.True(t, ok)
	assert.Equal(t, "Default", p.Name)

	// Save
	err = pm2.SavePersonas()
	require.NoError(t, err)

	// Load again
	pm3 := NewPersonaManager()
	err = pm3.LoadPersonas()
	require.NoError(t, err)

	p, ok = pm3.GetPersona("default")
	assert.True(t, ok)
	assert.Equal(t, "Default", p.Name) // Verified persisted reset
}
