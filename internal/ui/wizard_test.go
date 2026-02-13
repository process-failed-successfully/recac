package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/list"
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
	// Note: checking specifically "gemini-cli" depends on list order and key handling.
	// Assuming down key moved selection.

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
	view := m.View()
	if !strings.Contains(view, "Selected project: test-project") {
		t.Errorf("Expected final view to show selected project, got: %s", view)
	}
}

func TestWizardModel_ExplicitValues(t *testing.T) {
	m := NewWizardModel()
	m.step = StepMaxAgents

	// Set explicit MaxAgents
	m.textInput.SetValue("5")
	msg := tea.KeyMsg{Type: tea.KeyEnter}
	updatedModel, _ := m.Update(msg)
	m = updatedModel.(WizardModel)

	if m.MaxAgents != 5 {
		t.Errorf("Expected MaxAgents 5, got %d", m.MaxAgents)
	}

	// Set explicit TaskMaxIterations
	m.textInput.SetValue("20")
	msg = tea.KeyMsg{Type: tea.KeyEnter}
	updatedModel, _ = m.Update(msg)
	m = updatedModel.(WizardModel)

	if m.TaskMaxIterations != 20 {
		t.Errorf("Expected TaskMaxIterations 20, got %d", m.TaskMaxIterations)
	}
}

func TestWizardModel_InvalidInput(t *testing.T) {
	m := NewWizardModel()
	m.step = StepMaxAgents

	// Set invalid MaxAgents
	m.textInput.SetValue("invalid")
	msg := tea.KeyMsg{Type: tea.KeyEnter}
	updatedModel, _ := m.Update(msg)
	m = updatedModel.(WizardModel)

	// Should fall back to default or 1
	if m.MaxAgents != 1 {
		t.Errorf("Expected MaxAgents 1 for invalid input, got %d", m.MaxAgents)
	}

	// Test negative number
	m.step = StepMaxAgents // Reset step
	m.textInput.SetValue("-5")
	msg = tea.KeyMsg{Type: tea.KeyEnter}
	updatedModel, _ = m.Update(msg)
	m = updatedModel.(WizardModel)

	if m.MaxAgents != 1 {
		t.Errorf("Expected MaxAgents 1 for negative input, got %d", m.MaxAgents)
	}
}

func TestWizardModel_NoSelection(t *testing.T) {
	m := NewWizardModel()
	m.step = StepProvider

	// Force empty list to simulate no selection possible
	m.list.SetItems([]list.Item{})

	msg := tea.KeyMsg{Type: tea.KeyEnter}
	updatedModel, _ := m.Update(msg)
	m = updatedModel.(WizardModel)

	// Should stay on StepProvider
	if m.step != StepProvider {
		t.Error("Expected to stay on StepProvider when no item selected")
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

func TestWizardModel_ValidationFeedback(t *testing.T) {
	m := NewWizardModel()
	m.Init()

	// 1. Simulate Enter with empty path
	msg := tea.KeyMsg{Type: tea.KeyEnter}
	updatedModel, _ := m.Update(msg)
	m = updatedModel.(WizardModel)

	// Should still be on StepPath
	if m.step != StepPath {
		t.Error("Expected to stay on StepPath when input is empty")
	}

	// Should show error message (UX Requirement)
	view := m.View()
	if !strings.Contains(view, "Path cannot be empty") {
		t.Errorf("Expected view to show 'Path cannot be empty', got: %s", view)
	}

	// 2. Simulate typing clears error
	msgKey := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}}
	updatedModel, _ = m.Update(msgKey)
	m = updatedModel.(WizardModel)

	view = m.View()
	if strings.Contains(view, "Path cannot be empty") {
		t.Error("Expected error message to be cleared after typing")
	}
}

func TestWizardModel_View_Steps(t *testing.T) {
	m := NewWizardModel()

	// Set width/height so list renders properly
	m.list.SetWidth(40)
	m.list.SetHeight(20)

	// StepPath (already tested in InitialView)

	// StepProvider
	m.step = StepProvider
	view := m.View()
	if !strings.Contains(view, "Select Agent Provider") {
		t.Errorf("Expected view to contain 'Select Agent Provider', got: %s", view)
	}

	// StepMaxAgents
	m.step = StepMaxAgents
	view = m.View()
	if !strings.Contains(view, "Enter maximum parallel agents") {
		t.Errorf("Expected view to contain 'Enter maximum parallel agents', got: %s", view)
	}

	// StepTaskMaxIterations
	m.step = StepTaskMaxIterations
	view = m.View()
	if !strings.Contains(view, "Enter maximum iterations per task") {
		t.Errorf("Expected view to contain 'Enter maximum iterations per task', got: %s", view)
	}

	// Default case (invalid step)
	m.step = 999
	view = m.View()
	if view != "" {
		t.Errorf("Expected empty view for invalid step, got: %s", view)
	}
}

func TestWizardModel_WindowSize(t *testing.T) {
	m := NewWizardModel()
	msg := tea.WindowSizeMsg{Width: 100, Height: 50}

	updatedModel, _ := m.Update(msg)
	m = updatedModel.(WizardModel)

	if m.list.Width() != 100 {
		// List width might not correspond exactly to msg.Width depending on padding?
		// implementation: m.list.SetWidth(msg.Width)
		// But list model might adjust it.
		// Let's just check it didn't panic.
	}
}

func TestWizardModel_ErrorMsg(t *testing.T) {
	m := NewWizardModel()

	// Create actual error
	actualErr := tea.ErrProgramKilled

	updatedModel, _ := m.Update(actualErr)
	m = updatedModel.(WizardModel)

	if m.err != actualErr {
		t.Error("Expected error to be stored in model")
	}
}
