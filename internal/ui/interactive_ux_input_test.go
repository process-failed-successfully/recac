package ui

import (
	"testing"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

func TestInteractiveModel_MultilineInput(t *testing.T) {
	m := NewInteractiveModel(nil, "", "")

	// Setup: Simulate agent ready
	m.thinking = false
	m.statusMessage = ""

	// 1. Set initial text
	m.textarea.SetValue("Line 1")

	// 2. Simulate Alt+Enter
	msg := tea.KeyMsg{Type: tea.KeyEnter, Alt: true}

	// Ensure binding matches (sanity check for test env)
	if !key.Matches(msg, m.keys.Newline) {
		t.Fatal("Test environment issue: Alt+Enter msg does not match Newline binding")
	}

	updatedM, _ := m.Update(msg)
	m = updatedM.(InteractiveModel)

	// 3. Verify Newline Inserted
	expected := "Line 1\n"
	if m.textarea.Value() != expected {
		t.Errorf("Expected textarea value to be %q, got %q", expected, m.textarea.Value())
	}

	// 4. Verify NOT Submitted
	if m.thinking {
		t.Error("Model should not be thinking (submitted) after Alt+Enter")
	}
}
