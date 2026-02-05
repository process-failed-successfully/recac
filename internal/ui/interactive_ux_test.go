package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestInteractiveModel_CurrentSelectionIndicator(t *testing.T) {
	// Initialize with defaults (Gemini provider)
	m := NewInteractiveModel(nil, "gemini", "gemini-2.0-flash-auto")

	// 1. Verify Model Selection Indicator
	m.setMode(ModeModelSelect)

	items := m.list.Items()
	foundIndicator := false

	for _, item := range items {
		if mod, ok := item.(ModelItem); ok {
			if mod.Value == "gemini-2.0-flash-auto" {
				// Check if the name indicates it's current
				// We expect something like "Gemini 2.0 Flash (Auto) (Current)" or similar
				// Or check description
				if strings.Contains(mod.Title(), "(Current)") || strings.Contains(mod.Description(), "Currently active") {
					foundIndicator = true
				}
			}
		}
	}

	if !foundIndicator {
		t.Error("UX: Currently selected model in the list should have a visual indicator (e.g., '(Current)')")
	}

	// 2. Verify Agent Selection Indicator
	m.setMode(ModeAgentSelect)

	items = m.list.Items()
	foundIndicator = false

	for _, item := range items {
		if ag, ok := item.(AgentItem); ok {
			if ag.Value == "gemini" {
				if strings.Contains(ag.Title(), "(Current)") || strings.Contains(ag.Description(), "Currently active") {
					foundIndicator = true
				}
			}
		}
	}

	if !foundIndicator {
		t.Error("UX: Currently selected agent in the list should have a visual indicator")
	}
}

func TestInteractiveModel_CommandSearch(t *testing.T) {
	// Setup a command with a distinct description
	commands := []SlashCommand{
		{
			Name:        "/foo",
			Description: "Run the Bar feature",
			Action:      nil,
		},
		{
			Name:        "/baz",
			Description: "Something else",
			Action:      nil,
		},
	}

	m := NewInteractiveModel(commands, "gemini", "gemini-2.0-flash-auto")

	// Enter Command Mode
	m.setMode(ModeCmd)

	// Simulate user typing "/Bar" (matching description of /foo)
	m.textarea.SetValue("/Bar")

	// Trigger Update to run filtering logic.
	// We send a generic key message that isn't a special key, just to trigger the update loop.
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(" ")}

	// InteractiveModel.Update returns (tea.Model, tea.Cmd)
	updatedModel, _ := m.Update(msg)

	// Cast back to InteractiveModel
	finalM, ok := updatedModel.(InteractiveModel)
	if !ok {
		t.Fatal("Failed to cast updated model back to InteractiveModel")
	}

	// Verify filtered list
	items := finalM.list.Items()

	// Should match /foo because "Bar" is in "Run the Bar feature"
	found := false
	for _, item := range items {
		if cmd, ok := item.(CommandItem); ok {
			if cmd.Name == "/foo" {
				found = true
				break
			}
		}
	}

	if !found {
		t.Fatalf("Expected '/foo' to be found when searching for 'Bar' (description match). Items found: %v", items)
	}

	// Verify /baz is NOT found
	for _, item := range items {
		if cmd, ok := item.(CommandItem); ok {
			if cmd.Name == "/baz" {
				t.Error("Did not expect '/baz' to be in the filtered list")
			}
		}
	}
}
