package ui

import (
	"os"
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

	// Use a valid temporary directory for validation to pass
	validPath := t.TempDir()

	// Simulate typing the path
	for _, r := range validPath {
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
		// We need to cast back to WizardModel because Update returns tea.Model
		updatedModel, _ := m.Update(msg)
		m = updatedModel.(WizardModel)
	}

	// Verify text input value before Enter
	if m.textInput.Value() != validPath {
		t.Errorf("Expected text input value '%s', got '%s'", validPath, m.textInput.Value())
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
	// m.Path should now be the expanded validPath (which is absolute already from TempDir)
	if m.Path != validPath {
		t.Errorf("Expected path '%s', got '%s'", validPath, m.Path)
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
	if !strings.Contains(view, "Selected project: "+validPath) {
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
	if !strings.Contains(view, "Path does not exist") {
		t.Errorf("Expected view to show 'Path does not exist', got: %s", view)
	}

	// 2. Simulate typing clears error
	msgKey := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}}
	updatedModel, _ = m.Update(msgKey)
	m = updatedModel.(WizardModel)

	view = m.View()
	if strings.Contains(view, "Path does not exist") {
		t.Error("Expected error message to be cleared after typing")
	}
}

func TestWizardModel_PathValidation_Real(t *testing.T) {
	m := NewWizardModel()
	m.Init()

	// Use a non-existent path
	invalidPath := "/path/that/definitely/does/not/exist/12345"

	// Type the invalid path
	for _, r := range invalidPath {
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
		updatedModel, _ := m.Update(msg)
		m = updatedModel.(WizardModel)
	}

	// Simulate Enter
	msg := tea.KeyMsg{Type: tea.KeyEnter}
	updatedModel, _ := m.Update(msg)
	m = updatedModel.(WizardModel)

	// Should still be on StepPath
	if m.step != StepPath {
		t.Error("Expected to stay on StepPath when path is invalid")
	}

	// Check error message
	if m.errMsg != "Path does not exist" {
		t.Errorf("Expected error 'Path does not exist', got '%s'", m.errMsg)
	}

	// Try with a file instead of directory
	tmpDir := t.TempDir()
	tmpFile := tmpDir + "/testfile"
	err := os.WriteFile(tmpFile, []byte("content"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	// Reset input
	m.textInput.Reset()

	// Type the file path
	for _, r := range tmpFile {
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
		updatedModel, _ := m.Update(msg)
		m = updatedModel.(WizardModel)
	}

	// Simulate Enter
	msg = tea.KeyMsg{Type: tea.KeyEnter}
	updatedModel, _ = m.Update(msg)
	m = updatedModel.(WizardModel)

	// Should still be on StepPath
	if m.step != StepPath {
		t.Error("Expected to stay on StepPath when path is a file")
	}

	// Check error message
	if m.errMsg != "Path is not a directory" {
		t.Errorf("Expected error 'Path is not a directory', got '%s'", m.errMsg)
	}
}

func TestWizardModel_HelperText(t *testing.T) {
	m := NewWizardModel()
	m.step = StepMaxAgents // Fast forward to step

	view := m.View()
	// Check for the explicit default instruction
	if !strings.Contains(view, "Press Enter for default") {
		t.Errorf("Expected view to contain 'Press Enter for default', got: %s", view)
	}
}
