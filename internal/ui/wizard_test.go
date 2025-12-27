package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestWizardModel_InitialView(t *testing.T) {
	m := NewWizardModel()
	view := m.View()

	if !strings.Contains(view, "Project Setup") {
		t.Errorf("Expected view to contain 'Project Setup', got: %s", view)
	}
	if !strings.Contains(view, "Enter project directory") {
		t.Errorf("Expected view to ask for directory, got: %s", view)
	}
}

func TestWizardModel_Input(t *testing.T) {
	m := NewWizardModel()

	// Initialize the model (important for textinput blink etc, though not strictly needed for logic test)
	m.Init()

	// Simulate typing "test-project"
	input := "test-project"
	for _, r := range input {
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
		// We need to cast back to WizardModel because Update returns tea.Model
		updatedModel, _ := m.Update(msg)
		m = updatedModel.(WizardModel)
	}

	// Verify text input value before Enter
	if m.textInput.Value() != "test-project" {
		t.Errorf("Expected text input value 'test-project', got '%s'", m.textInput.Value())
	}

	// Simulate Enter
	msg := tea.KeyMsg{Type: tea.KeyEnter}
	updatedModel, _ := m.Update(msg)
	m = updatedModel.(WizardModel)

	if !m.done {
		t.Error("Expected model to be done after Enter")
	}
	if m.Path != "test-project" {
		t.Errorf("Expected path 'test-project', got '%s'", m.Path)
	}

	// Check final view
	view := m.View()
		if !strings.Contains(view, "Selected project: test-project") {
			t.Errorf("Expected final view to show selected project, got: %s", view)
		}
	}
	
	func TestWizardModel_Quit(t *testing.T) {
		m := NewWizardModel()
	
		// Simulate Ctrl+C
		msg := tea.KeyMsg{Type: tea.KeyCtrlC}
		_, cmd := m.Update(msg)
		
		if cmd == nil {
			t.Fatal("Expected a command after Ctrl+C, got nil")
		}
		
		// We can't easily check if it's tea.Quit because it's an internal type/function in bubbletea
		// but we can check the behavior in our app if we want.
		// Actually, tea.Quit() returns a tea.Cmd which is a func() tea.Msg.
	}
	