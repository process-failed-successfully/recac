package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestWizardModel_InvalidNumericInput(t *testing.T) {
	m := NewWizardModel()
	m.Init()
	m.step = StepMaxAgents // Jump to MaxAgents step

	// 1. Simulate typing invalid number "abc"
	input := "abc"
	for _, r := range input {
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
		updatedModel, _ := m.Update(msg)
		m = updatedModel.(WizardModel)
	}

	// 2. Simulate Enter
	msg := tea.KeyMsg{Type: tea.KeyEnter}
	updatedModel, _ := m.Update(msg)
	m = updatedModel.(WizardModel)

	// Should transition to next step, but MaxAgents should be default (1)
	if m.step != StepTaskMaxIterations {
		t.Error("Expected to transition to StepTaskMaxIterations even with invalid input (fallback to default)")
	}
	if m.MaxAgents != 1 {
		t.Errorf("Expected MaxAgents 1 (default), got %d", m.MaxAgents)
	}

	// 3. Test TaskMaxIterations with invalid input "-1"
	// Simulate typing "-1"
	input = "-1"
	m.textInput.Reset()
	for _, r := range input {
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
		updatedModel, _ := m.Update(msg)
		m = updatedModel.(WizardModel)
	}

	msg = tea.KeyMsg{Type: tea.KeyEnter}
	updatedModel, _ = m.Update(msg)
	m = updatedModel.(WizardModel)

	if !m.done {
		t.Error("Expected model to be done")
	}
	// Logic says if n < 1 { n = 1 } or similar check?
	// In WizardModel.Update: if n < 1 { n = 1 }
	if m.TaskMaxIterations != 1 { // Should be 1 because input was -1
		t.Errorf("Expected TaskMaxIterations 1 (clamped), got %d", m.TaskMaxIterations)
	}
}

func TestWizardModel_WindowResize(t *testing.T) {
	m := NewWizardModel()

	// Simulate window resize
	msg := tea.WindowSizeMsg{Width: 100, Height: 50}
	updatedModel, _ := m.Update(msg)
	m = updatedModel.(WizardModel)

	if m.list.Width() != 100 {
		t.Errorf("Expected list width 100, got %d", m.list.Width())
	}
}

func TestWizardModel_ErrorHandling(t *testing.T) {
	m := NewWizardModel()

	testErr := testError("test error")

	updatedModel, _ := m.Update(testErr)
	m = updatedModel.(WizardModel)

	if m.err == nil || m.err.Error() != "test error" {
		t.Errorf("Expected error 'test error', got %v", m.err)
	}
}

type testError string

func (e testError) Error() string { return string(e) }

func TestWizardModel_ViewStates(t *testing.T) {
	m := NewWizardModel()
	m.list.SetWidth(40)
	m.list.SetHeight(20)

	// StepPath (initial)
	view := m.View()
	if !strings.Contains(view, "Project Setup") {
		t.Error("Missing Project Setup in view")
	}

	// StepProvider
	m.step = StepProvider
	view = m.View()
	if !strings.Contains(view, "Select Agent Provider") { // List title
		t.Error("Missing Select Agent Provider in view")
	}

	// StepMaxAgents
	m.step = StepMaxAgents
	view = m.View()
	if !strings.Contains(view, "Agent Configuration") {
		t.Error("Missing Agent Configuration in view")
	}

	// StepTaskMaxIterations
	m.step = StepTaskMaxIterations
	view = m.View()
	if !strings.Contains(view, "Agent Configuration") {
		t.Error("Missing Agent Configuration in view")
	}

	// Done
	m.done = true
	m.Path = "/tmp"
	m.Provider = "gemini"
	view = m.View()
	if !strings.Contains(view, "Selected project: /tmp") {
		t.Error("Missing summary in done view")
	}

	// Invalid Step (should return empty string or safe fallback)
	m.done = false
	m.step = 999
	view = m.View()
	if view != "" {
		t.Errorf("Expected empty view for invalid step, got: %s", view)
	}
}
