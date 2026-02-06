package agent

import (
	"context"
	"path/filepath"
	"testing"
)

// MockPersonaAgent is a specific mock for testing persona wrapper prompts
type MockPersonaAgent struct {
	LastPrompt string
}

func (m *MockPersonaAgent) Send(ctx context.Context, prompt string) (string, error) {
	m.LastPrompt = prompt
	return "mock response", nil
}

func (m *MockPersonaAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	m.LastPrompt = prompt
	return "mock response", nil
}

func TestPersonaManager_CRUD(t *testing.T) {
	tmpDir := t.TempDir()
	personasFile := filepath.Join(tmpDir, "personas.yaml")
	activeFile := filepath.Join(tmpDir, "active_persona")

	t.Setenv("RECAC_PERSONAS_FILE", personasFile)
	t.Setenv("RECAC_ACTIVE_PERSONA_FILE", activeFile)

	pm := NewPersonaManager()

	// 1. List Default
	personas := pm.ListPersonas()
	foundDefault := false
	for _, p := range personas {
		if p.Name == "Default" {
			foundDefault = true
			break
		}
	}
	if !foundDefault {
		t.Error("Default persona not found")
	}

	// 2. Add Custom
	custom := Persona{
		Name:         "CustomTester",
		Description:  "A test persona",
		SystemPrompt: "You are a test.",
	}
	if err := pm.SaveCustomPersona(custom); err != nil {
		t.Fatalf("Failed to save custom persona: %v", err)
	}

	// 3. Verify it's persisted by creating new manager
	pm2 := NewPersonaManager()
	p, ok := pm2.GetPersona("CustomTester")
	if !ok {
		t.Error("Custom persona not persisted")
	}
	if p.SystemPrompt != custom.SystemPrompt {
		t.Errorf("Expected prompt %s, got %s", custom.SystemPrompt, p.SystemPrompt)
	}

	// 4. Set Active
	if err := pm.SetActivePersona("CustomTester"); err != nil {
		t.Fatalf("Failed to set active persona: %v", err)
	}

	// Verify persistence of active persona
	pm3 := NewPersonaManager()
	active := pm3.GetActivePersona()
	if active.Name != "CustomTester" {
		t.Errorf("Expected active persona CustomTester, got %s", active.Name)
	}

	// 5. Delete
	if err := pm.DeleteCustomPersona("CustomTester"); err != nil {
		t.Fatalf("Failed to delete persona: %v", err)
	}

	if _, ok := pm.GetPersona("CustomTester"); ok {
		t.Error("Custom persona should be deleted")
	}
}

func TestPersonaAgentWrapper(t *testing.T) {
	mock := &MockPersonaAgent{}
	wrapper := &PersonaAgentWrapper{
		Agent:        mock,
		SystemPrompt: "Be polite.",
	}

	prompt := "Hello"
	wrapper.Send(context.Background(), prompt)

	expected := "SYSTEM INSTRUCTION: Be polite.\n\nUSER PROMPT: Hello"
	if mock.LastPrompt != expected {
		t.Errorf("Expected prompt:\n%s\nGot:\n%s", expected, mock.LastPrompt)
	}
}
