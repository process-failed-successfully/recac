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

	// Set window size
	m.list.SetWidth(80)
	m.list.SetHeight(20)

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

	// Simulate Enter to set path
	msg := tea.KeyMsg{Type: tea.KeyEnter}
	updatedModel, _ := m.Update(msg)
	m = updatedModel.(WizardModel)

	if m.done {
		t.Error("Expected model NOT to be done after first Enter (Path step)")
	}
	if m.step != StepProvider {
		t.Error("Expected to transition to StepProvider")
	}
	if m.Path != "test-project" {
		t.Errorf("Expected path 'test-project', got '%s'", m.Path)
	}

	// Verify View for StepProvider
	view := m.View()
	if !strings.Contains(view, "Gemini") {
		t.Errorf("Expected provider list in view, got: %s", view)
	}

	// Step 2: Provider Selection
	// Default selected item is usually the first one ("Gemini") or index 0.
	// Let's verify list is active.
	if len(m.list.Items()) == 0 {
		t.Error("Expected provider list to have items")
	}

	// Simulate selecting the second item ("Gemini CLI")
	// Down key
	msg = tea.KeyMsg{Type: tea.KeyDown}
	updatedModel, _ = m.Update(msg)
	m = updatedModel.(WizardModel)

	// Simulate Enter to select provider
	msg = tea.KeyMsg{Type: tea.KeyEnter}
	updatedModel, _ = m.Update(msg)
	m = updatedModel.(WizardModel)

	if m.done {
		t.Error("Expected model NOT to be done after second Enter (Provider step)")
	}
	if m.step != StepMaxAgents {
		t.Error("Expected to transition to StepMaxAgents")
	}
	if m.Provider != "gemini-cli" {
		t.Errorf("Expected provider 'gemini-cli', got '%s'", m.Provider)
	}

	// Verify View for StepMaxAgents
	view = m.View()
	if !strings.Contains(view, "Enter maximum parallel agents") {
		t.Errorf("Expected Max Agents prompt in view, got: %s", view)
	}

	// Step 3: Max Agents Selection
	// Simulate Enter with default 1
	msg = tea.KeyMsg{Type: tea.KeyEnter}
	updatedModel, _ = m.Update(msg)
	m = updatedModel.(WizardModel)

	if m.done {
		t.Error("Expected model NOT to be done after third Enter (MaxAgents step)")
	}
	if m.step != StepTaskMaxIterations {
		t.Error("Expected to transition to StepTaskMaxIterations")
	}

	// Verify View for StepTaskMaxIterations
	view = m.View()
	if !strings.Contains(view, "Enter maximum iterations per task") {
		t.Errorf("Expected Max Iterations prompt in view, got: %s", view)
	}

	// Step 4: Task Max Iterations
	// Simulate Enter with default 10
	msg = tea.KeyMsg{Type: tea.KeyEnter}
	updatedModel, _ = m.Update(msg)
	m = updatedModel.(WizardModel)

	if !m.done {
		t.Error("Expected model to be done after fourth Enter (TaskMaxIterations step)")
	}
	if m.MaxAgents != 1 {
		t.Errorf("Expected MaxAgents 1, got %d", m.MaxAgents)
	}

	// Check final view
	view = m.View()
	if !strings.Contains(view, "Selected project: test-project") {
		t.Errorf("Expected final view to show selected project, got: %s", view)
	}
	if !strings.Contains(view, "Selected provider: gemini-cli") {
		t.Errorf("Expected final view to show selected provider, got: %s", view)
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
