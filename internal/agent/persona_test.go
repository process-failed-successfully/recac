package agent

import (
	"os"
	"path/filepath"
	"sort"
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
	t.Setenv("RECAC_PERSONAS_FILE", tmpFile)

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
	t.Setenv("RECAC_PERSONAS_FILE", tmpFile)

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

func TestPersonaManager_ListPersonas(t *testing.T) {
	pm := NewPersonaManager()

	// Test ListPersonas (map)
	personas := pm.ListPersonas()
	assert.NotEmpty(t, personas)
	assert.Contains(t, personas, "default")
	assert.Contains(t, personas, "security")

	// Test ListSorted (slice of keys)
	sorted := pm.ListSorted()
	assert.NotEmpty(t, sorted)
	assert.Equal(t, len(personas), len(sorted))
	assert.True(t, sort.StringsAreSorted(sorted))

	// Verify contents match keys
	for _, key := range sorted {
		_, ok := personas[key]
		assert.True(t, ok)
	}
}

func TestGetPersonasFilePath_Default(t *testing.T) {
	// Ensure env var is unset for this test
	t.Setenv("RECAC_PERSONAS_FILE", "")

	path, err := getPersonasFilePath()
	require.NoError(t, err)

	home, err := os.UserHomeDir()
	require.NoError(t, err)
	expected := filepath.Join(home, ".recac", "personas.yaml")
	assert.Equal(t, expected, path)
}
