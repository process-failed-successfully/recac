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

func TestPersonaManager_MissingHome(t *testing.T) {
	// Unset RECAC_PERSONAS_FILE
	origEnv := os.Getenv("RECAC_PERSONAS_FILE")
	os.Unsetenv("RECAC_PERSONAS_FILE")
	defer os.Setenv("RECAC_PERSONAS_FILE", origEnv)

	// Mock missing home by unsetting HOME
	// Note: os.UserHomeDir behavior depends on OS.
	// On Linux, it checks $HOME.
	origHome := os.Getenv("HOME")
	os.Unsetenv("HOME")
	defer os.Setenv("HOME", origHome)

	pm := NewPersonaManager()

	// Load should succeed (return nil) but load nothing custom
	err := pm.LoadPersonas()
	assert.NoError(t, err)

	// Defaults should still be present
	_, ok := pm.GetPersona("default")
	assert.True(t, ok)

	// Save should fail
	err = pm.SavePersonas()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "home directory not found")
}

func TestPersonaManager_BadHome(t *testing.T) {
	// Set HOME to a non-directory file to trigger filesystem errors
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", "/dev/null")
	defer os.Setenv("HOME", origHome)

	// Ensure env var is unset so we use default path logic
	origEnv := os.Getenv("RECAC_PERSONAS_FILE")
	os.Unsetenv("RECAC_PERSONAS_FILE")
	defer os.Setenv("RECAC_PERSONAS_FILE", origEnv)

	pm := NewPersonaManager()

	// Should fail to stat/read but return nil (graceful fallback)
	err := pm.LoadPersonas()
	assert.NoError(t, err)

	// Save should fail because mkdir fails
	err = pm.SavePersonas()
	assert.Error(t, err)
}
