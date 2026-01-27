package ui

import (
	"strings"
	"testing"
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
