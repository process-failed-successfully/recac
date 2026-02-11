package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestWizardModel_InitialView(t *testing.T) {
	m := NewWizardModel()
	view := m.View()

	if !strings.Contains(view, "Project Setup [1/4]") {
		t.Errorf("Expected view to contain 'Project Setup [1/4]', got: %s", view)
	}
	if !strings.Contains(view, "Enter project directory") {
		t.Errorf("Expected view to ask for directory, got: %s", view)
	}
}

func TestWizardModel_Input(t *testing.T) {
	m := NewWizardModel()

	// Initialize the model
	m.Init()

	// Simulate typing "test-project"
	input := "test-project"
	for _, r := range input {
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
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
	if len(m.list.Items()) == 0 {
		t.Error("Expected provider list to have items")
	}

	// Verify Step 2 Title
	if !strings.Contains(m.list.Title, "[2/4]") {
		t.Errorf("Expected list title to contain '[2/4]', got: %s", m.list.Title)
	}

	// Simulate selecting the second item ("Gemini CLI")
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

	// Verify Step 3 View
	view := m.View()
	if !strings.Contains(view, "Agent Configuration [3/4]") {
		t.Errorf("Expected view to contain 'Agent Configuration [3/4]', got: %s", view)
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

	// Verify Step 4 View
	view = m.View()
	if !strings.Contains(view, "Agent Configuration [4/4]") {
		t.Errorf("Expected view to contain 'Agent Configuration [4/4]', got: %s", view)
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

func TestWizardModel_NumericValidation(t *testing.T) {
	m := NewWizardModel()
	m.step = StepMaxAgents // Fast forward

	// 1. Simulate invalid input "abc"
	input := "abc"
	for _, r := range input {
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
		updatedModel, _ := m.Update(msg)
		m = updatedModel.(WizardModel)
	}

	msg := tea.KeyMsg{Type: tea.KeyEnter}
	updatedModel, _ := m.Update(msg)
	m = updatedModel.(WizardModel)

	// Should not transition
	if m.step != StepMaxAgents {
		t.Error("Expected to stay on StepMaxAgents with invalid input")
	}
	if !strings.Contains(m.View(), "Please enter a valid number") {
		t.Error("Expected validation error message")
	}

	// 2. Simulate invalid input "-5"
	// Clear first (simulated by just resetting textInput value or backspacing, but easier to create new model or just re-type valid input which overwrites error msg)
	// But textInput.Value is "abc". Let's clear it.
	m.textInput.SetValue("-5")

	msg = tea.KeyMsg{Type: tea.KeyEnter}
	updatedModel, _ = m.Update(msg)
	m = updatedModel.(WizardModel)

	if m.step != StepMaxAgents {
		t.Error("Expected to stay on StepMaxAgents with negative input")
	}

	// 3. Simulate valid input "5"
	m.textInput.SetValue("5")
	// Clear error message happens on next key press or inside Update loop if we were typing.
	// But since we set value directly, we call Update with Enter.
	// Wait, Update clears errMsg on KeyMsg (except Enter).
	// So if we just press Enter, errMsg might persist if logic isn't careful.
	// But logic says:
	// if err != nil || n < 1 { m.errMsg = ... return }
	// m.MaxAgents = n
	// return ...
	// So if input is valid, it transitions and returns. It doesn't clear errMsg explicitly but we transition to next step which re-renders.
	// The next step View() checks m.errMsg. Since we reused 'm', errMsg is still set?
	// Ah, WizardModel is a struct (value), so 'm' in Update is a copy, but we return updated 'm'.
	// Yes, errMsg is a field in 'm'.
	// If we transition, we should probably clear errMsg.
	// Let's check the code:
	// m.step = StepTaskMaxIterations
	// m.textInput.Reset()
	// ...
	// It does NOT clear errMsg explicitly.
	// But wait, Update says:
	// case tea.KeyMsg: if msg.Type != tea.KeyEnter { m.errMsg = "" }
	// So typing the number would clear it.
	// But here we set Value directly.
	// Let's simulate typing "5" key by key to be realistic.

	m.textInput.Reset() // Clear "abc" or "-5"
	// Typing '5' will clear errMsg because it's a KeyRunes
	msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'5'}}
	updatedModel, _ = m.Update(msg)
	m = updatedModel.(WizardModel)

	if m.errMsg != "" {
		t.Error("Expected error message to be cleared after typing")
	}

	msg = tea.KeyMsg{Type: tea.KeyEnter}
	updatedModel, _ = m.Update(msg)
	m = updatedModel.(WizardModel)

	if m.step != StepTaskMaxIterations {
		t.Error("Expected to transition to StepTaskMaxIterations with valid input")
	}
	if m.MaxAgents != 5 {
		t.Errorf("Expected MaxAgents 5, got %d", m.MaxAgents)
	}
}

func TestWizardModel_Quit(t *testing.T) {
	m := NewWizardModel()
	msg := tea.KeyMsg{Type: tea.KeyCtrlC}
	_, cmd := m.Update(msg)
	if cmd == nil {
		t.Fatal("Expected a command after Ctrl+C")
	}
}

func TestWizardModel_ValidationFeedback(t *testing.T) {
	m := NewWizardModel()
	m.Init()

	// 1. Simulate Enter with empty path
	msg := tea.KeyMsg{Type: tea.KeyEnter}
	updatedModel, _ := m.Update(msg)
	m = updatedModel.(WizardModel)

	if m.step != StepPath {
		t.Error("Expected to stay on StepPath when input is empty")
	}

	view := m.View()
	if !strings.Contains(view, "Path cannot be empty") {
		t.Errorf("Expected view to show 'Path cannot be empty', got: %s", view)
	}
}

func TestWizardModel_HelperText(t *testing.T) {
	m := NewWizardModel()
	m.step = StepMaxAgents

	view := m.View()
	if !strings.Contains(view, "Press Enter for default") {
		t.Errorf("Expected view to contain 'Press Enter for default', got: %s", view)
	}
}
