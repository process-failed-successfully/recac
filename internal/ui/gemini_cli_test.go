package ui

import (
	"testing"
)

func TestInteractiveModel_GeminiCLIModels(t *testing.T) {
	m := NewInteractiveModel(nil)
	m.currentAgent = "gemini-cli"
	m.setMode(ModeModelSelect)

	items := m.list.Items()
	if len(items) == 0 {
		t.Error("Model list should not be empty for gemini-cli")
	}

	foundAuto := false
	foundPro := false
	for _, item := range items {
		if mod, ok := item.(ModelItem); ok {
			if mod.Value == "auto" {
				foundAuto = true
			}
			if mod.Value == "pro" {
				foundPro = true
			}
		}
	}

	if !foundAuto {
		t.Error("Gemini CLI list should contain 'auto'")
	}
	if !foundPro {
		t.Error("Gemini CLI list should contain 'pro'")
	}
}
