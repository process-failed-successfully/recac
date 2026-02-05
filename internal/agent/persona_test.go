package agent

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadSavePersonas(t *testing.T) {
	// Create temp dir
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "personas.yaml")

	// 1. Load empty
	personas, err := LoadPersonas(path)
	if err != nil {
		t.Fatalf("LoadPersonas failed: %v", err)
	}
	if len(personas) != 0 {
		t.Errorf("Expected empty personas, got %d", len(personas))
	}

	// 2. Save
	personas["test"] = Persona{
		Name:        "Test Persona",
		Description: "A test persona",
		SystemPrompt: "You are a test.",
	}

	if err := SavePersonas(path, personas); err != nil {
		t.Fatalf("SavePersonas failed: %v", err)
	}

	// 3. Load again
	loaded, err := LoadPersonas(path)
	if err != nil {
		t.Fatalf("LoadPersonas failed: %v", err)
	}
	if len(loaded) != 1 {
		t.Errorf("Expected 1 persona, got %d", len(loaded))
	}
	if p, ok := loaded["test"]; !ok {
		t.Error("Missing 'test' persona")
	} else {
		if p.Name != "Test Persona" {
			t.Errorf("Expected name 'Test Persona', got '%s'", p.Name)
		}
	}
}

func TestSavePersonas_Mkdir(t *testing.T) {
	tmpDir := t.TempDir()
	// Use a nested path to test MkdirAll
	path := filepath.Join(tmpDir, "nested", "dir", "personas.yaml")

	personas := map[string]Persona{
		"test": {Name: "Test"},
	}

	if err := SavePersonas(path, personas); err != nil {
		t.Fatalf("SavePersonas failed: %v", err)
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("File was not created")
	}
}
