package ui

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"recac/internal/agent"

	tea "github.com/charmbracelet/bubbletea"
)

// CapturingMockAgent for testing prompt injection
type CapturingMockAgent struct {
	LastPrompt string
	Response   string
}

func (m *CapturingMockAgent) Send(ctx context.Context, prompt string) (string, error) {
	m.LastPrompt = prompt
	return m.Response, nil
}

func (m *CapturingMockAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	m.LastPrompt = prompt
	onChunk(m.Response)
	return m.Response, nil
}

func TestInteractiveModel_Persona_Initialization(t *testing.T) {
	// Isolate environment
	t.Setenv("RECAC_PERSONAS_FILE", filepath.Join(t.TempDir(), "nonexistent.yaml"))

	m := NewInteractiveModel(nil, "", "")

	if m.personaManager == nil {
		t.Error("PersonaManager should be initialized")
	}

	if m.currentPersona != "default" {
		t.Errorf("Expected default persona to be 'default', got '%s'", m.currentPersona)
	}

	// Check if /persona command exists
	found := false
	for _, c := range m.commands {
		if c.Name == "/persona" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected /persona command to be registered")
	}
}

func TestInteractiveModel_Persona_Command(t *testing.T) {
	// Isolate environment
	t.Setenv("RECAC_PERSONAS_FILE", filepath.Join(t.TempDir(), "nonexistent.yaml"))

	m := NewInteractiveModel(nil, "", "")

	// Execute /persona
	m.textarea.SetValue("/persona")
	msg := tea.KeyMsg{Type: tea.KeyEnter}

	updatedM, _ := m.Update(msg)
	m = updatedM.(InteractiveModel)

	if m.mode != ModePersonaSelect {
		t.Errorf("Expected mode to be ModePersonaSelect, got %v", m.mode)
	}
	if !m.showList {
		t.Error("Expected list to be shown")
	}

	// Check list items are personas
	if len(m.list.Items()) == 0 {
		t.Error("Persona list should not be empty")
	}
	if _, ok := m.list.Items()[0].(PersonaItem); !ok {
		t.Error("List items should be of type PersonaItem")
	}
}

func TestInteractiveModel_Persona_Selection(t *testing.T) {
	// Isolate environment
	t.Setenv("RECAC_PERSONAS_FILE", filepath.Join(t.TempDir(), "nonexistent.yaml"))

	m := NewInteractiveModel(nil, "", "")

	// Inject 'junior' explicitly to ensure test stability
	m.personaManager.AddPersona("junior", agent.Persona{
		Name:         "Junior Developer",
		Description:  "Needs simple explanations.",
		SystemPrompt: "You are a junior developer...",
	})

	m.setMode(ModePersonaSelect)

	// Find "junior" persona index
	var targetIndex int
	found := false
	for i, item := range m.list.Items() {
		if p, ok := item.(PersonaItem); ok && p.Value == "junior" {
			targetIndex = i
			found = true
			break
		}
	}
	if !found {
		t.Fatal("Could not find 'junior' persona for test")
	}

	m.list.Select(targetIndex)

	// Press Enter
	msg := tea.KeyMsg{Type: tea.KeyEnter}
	updatedM, _ := m.Update(msg)
	m = updatedM.(InteractiveModel)

	if m.currentPersona != "junior" {
		t.Errorf("Expected currentPersona to be 'junior', got '%s'", m.currentPersona)
	}
	if m.mode != ModeChat {
		t.Error("Expected to return to ModeChat")
	}
}

func TestInteractiveModel_Persona_PromptInjection(t *testing.T) {
	// Isolate environment
	t.Setenv("RECAC_PERSONAS_FILE", filepath.Join(t.TempDir(), "nonexistent.yaml"))

	m := NewInteractiveModel(nil, "", "")
	mockAgent := &CapturingMockAgent{Response: "OK"}
	m.activeAgent = mockAgent

	// Inject 'teacher' explicitly
	expectedSystemPrompt := "You are an expert Computer Science Teacher. Instead of giving the answer directly, you often ask guiding questions."
	m.personaManager.AddPersona("teacher", agent.Persona{
		Name:         "The Teacher",
		Description:  "Uses Socratic method.",
		SystemPrompt: expectedSystemPrompt,
	})

	// Set persona to 'teacher'
	m.currentPersona = "teacher"

	userPrompt := "Explain recursion"
	cmd := m.generateResponse(userPrompt)

	if cmd == nil {
		t.Fatal("Expected command from generateResponse")
	}

	// Execute command ONCE to start the stream
	msg := cmd()

	// Wait for stream to start
	streamMsg, ok := msg.(AgentStreamStartMsg)
	if !ok {
		t.Fatalf("Expected AgentStreamStartMsg, got %T", msg)
	}

	// Read from the channel to ensure execution completes
	// Depending on implementation, we might need to drain the channel
	for range streamMsg.ChunkChan {
		// Draining
	}

	// Now LastPrompt should be set on the mock agent
	if !strings.Contains(mockAgent.LastPrompt, expectedSystemPrompt) {
		t.Errorf("System prompt injection failed.\nExpected to contain: %s\nGot: %s", expectedSystemPrompt, mockAgent.LastPrompt)
	}

	if !strings.Contains(mockAgent.LastPrompt, userPrompt) {
		t.Errorf("User prompt missing.\nExpected to contain: %s\nGot: %s", userPrompt, mockAgent.LastPrompt)
	}
}
