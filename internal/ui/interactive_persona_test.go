package ui

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"recac/internal/agent"
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
	m := NewInteractiveModel(nil, "", "")

	// Inject test persona to avoid relying on defaults
	testPersona := agent.Persona{
		Name:         "Test Persona",
		Description:  "For testing",
		SystemPrompt: "You are a test.",
	}
	m.personaManager.AddPersona("test-persona", testPersona)

	// Update mode which should trigger list refresh (if properly implemented, or trigger it manually)
	m.setMode(ModePersonaSelect)

	// Find "test-persona" persona index
	var targetIndex int
	found := false
	for i, item := range m.list.Items() {
		if p, ok := item.(PersonaItem); ok && p.Value == "test-persona" {
			targetIndex = i
			found = true
			break
		}
	}
	if !found {
		t.Fatal("Could not find 'test-persona' for test")
	}

	m.list.Select(targetIndex)

	// Press Enter
	msg := tea.KeyMsg{Type: tea.KeyEnter}
	updatedM, _ := m.Update(msg)
	m = updatedM.(InteractiveModel)

	if m.currentPersona != "test-persona" {
		t.Errorf("Expected currentPersona to be 'test-persona', got '%s'", m.currentPersona)
	}
	if m.mode != ModeChat {
		t.Error("Expected to return to ModeChat")
	}
}

func TestInteractiveModel_Persona_PromptInjection(t *testing.T) {
	m := NewInteractiveModel(nil, "", "")
	mockAgent := &CapturingMockAgent{Response: "OK"}
	m.activeAgent = mockAgent

	// Set persona to 'test-teacher' which has a specific system prompt
	testPersona := agent.Persona{
		Name:         "Test Teacher",
		Description:  "For testing injection",
		SystemPrompt: "You are a test-teacher. Injected content.",
	}
	m.personaManager.AddPersona("test-teacher", testPersona)
	m.currentPersona = "test-teacher"

	userPrompt := "Explain recursion"
	cmd := m.generateResponse(userPrompt)

	// Run the command to trigger the goroutine
	if cmd != nil {
		cmd()
	}

	// Wait, the goroutine runs:
	// _, err := m.activeAgent.SendStream(...)

	// We need to ensure that runs.
	// We can't easily wait for it without modifying code or using a channel in the test.
	// But generateResponse returns a AgentStreamStartMsg containing channels.

	msg := cmd()
	streamMsg, ok := msg.(AgentStreamStartMsg)
	if !ok {
		t.Fatal("Expected AgentStreamStartMsg")
	}

	// Read from the channel to ensure execution
	<-streamMsg.ChunkChan

	// Now LastPrompt should be set
	if !strings.Contains(mockAgent.LastPrompt, "You are a test-teacher. Injected content.") {
		t.Errorf("System prompt injection failed. Prompt was: %s", mockAgent.LastPrompt)
	}

	if !strings.Contains(mockAgent.LastPrompt, userPrompt) {
		t.Errorf("User prompt missing. Prompt was: %s", mockAgent.LastPrompt)
	}
}
