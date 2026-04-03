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

func TestPersonaManager_LoadSave_Errors(t *testing.T) {
	pm := NewPersonaManager()

	tmpDir := t.TempDir()
	personasFile := filepath.Join(tmpDir, "personas.yaml")
	os.Setenv("RECAC_PERSONAS_FILE", personasFile)
	defer os.Unsetenv("RECAC_PERSONAS_FILE")

	// Test invalid JSON on load
	err := pm.SavePersonas()
	if err != nil {
		t.Fatalf("failed to save personas: %v", err)
	}

	err = os.MkdirAll(filepath.Dir(personasFile), 0755)
	if err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}

	err = os.WriteFile(personasFile, []byte("invalid json"), 0644)
	if err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	err = pm.LoadPersonas()
	if err == nil {
		t.Fatalf("expected error on load due to invalid JSON, got none")
	}

	// Test mkdir error on save
	os.RemoveAll(filepath.Dir(personasFile))
	err = os.WriteFile(filepath.Dir(personasFile), []byte("file"), 0644)
	if err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	err = pm.SavePersonas()
	if err == nil {
		t.Fatalf("expected error on save due to mkdir failure, got none")
	}
}

func TestPersonaManager_ListPersonas(t *testing.T) {
	pm := NewPersonaManager()

	pm.AddPersona("personaB", Persona{Name: "personaB"})
	pm.AddPersona("personaA", Persona{Name: "personaA"})

	personas := pm.ListPersonas()
	if len(personas) < 2 {
		t.Fatalf("expected at least 2 personas, got %d", len(personas))
	}

	sorted := pm.ListSorted()
	if len(sorted) < 2 {
		t.Fatalf("expected at least 2 sorted personas, got %d", len(sorted))
	}
}

func TestGetPersonasFilePath(t *testing.T) {
	origEnv := os.Getenv("RECAC_PERSONAS_FILE")
	defer os.Setenv("RECAC_PERSONAS_FILE", origEnv)

	origHome := os.Getenv("HOME")
	defer os.Setenv("HOME", origHome)

	tmpHome := t.TempDir()

	tests := []struct {
		name        string
		setup       func()
		expected    string
		expectError bool
	}{
		{
			name: "Env var set",
			setup: func() {
				os.Setenv("RECAC_PERSONAS_FILE", "/tmp/test_personas.yaml")
			},
			expected: "/tmp/test_personas.yaml",
			expectError: false,
		},
		{
			name: "Env var not set",
			setup: func() {
				os.Unsetenv("RECAC_PERSONAS_FILE")
				os.Setenv("HOME", tmpHome)
			},
			expected: filepath.Join(tmpHome, ".recac", "personas.yaml"),
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			path, err := getPersonasFilePath()
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, path)
			}
		})
	}
}
