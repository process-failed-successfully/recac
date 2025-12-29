package ui

import (
	"testing"
)

func TestInteractiveModel_CursorCLIModels(t *testing.T) {
	m := NewInteractiveModel(nil)
	models, ok := m.agentModels["cursor-cli"]
	if !ok {
		t.Fatal("cursor-cli should be in agentModels")
	}

	foundAuto := false
	for _, mode := range models {
		if mode.Value == "auto" {
			foundAuto = true
			break
		}
	}

	if !foundAuto {
		t.Error("cursor-cli models should include 'auto'")
	}
}
