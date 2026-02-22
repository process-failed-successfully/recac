package main

import (
	"os"
	"path/filepath"
	"testing"

	"recac/internal/architecture"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadArchitecture(t *testing.T) {
	tempDir := t.TempDir()
	yamlContent := `
system_name: TestSystem
components:
  - id: comp-1
    type: service
    description: A test service
`
	path := filepath.Join(tempDir, "architecture.yaml")
	err := os.WriteFile(path, []byte(yamlContent), 0644)
	require.NoError(t, err)

	arch, err := loadArchitecture(path)
	require.NoError(t, err)
	assert.Equal(t, "TestSystem", arch.SystemName)
	assert.Len(t, arch.Components, 1)
	assert.Equal(t, "comp-1", arch.Components[0].ID)
}

func TestArchitectModel_Update_Focus(t *testing.T) {
	arch := &architecture.SystemArchitecture{
		Components: []architecture.Component{
			{ID: "comp-1", Type: "service"},
			{ID: "comp-2", Type: "worker"},
		},
	}
	model := initialModel(arch)

	// Initial state: focus on list
	assert.Equal(t, focusList, model.focused)

	// Press Tab -> focus details
	model, cmd := updateModel(model, tea.KeyMsg{Type: tea.KeyTab})
	assert.Nil(t, cmd) // cmd is nil for focus switch
	assert.Equal(t, focusDetails, model.focused)

	// Press Tab -> focus list
	model, cmd = updateModel(model, tea.KeyMsg{Type: tea.KeyTab})
	assert.Nil(t, cmd)
	assert.Equal(t, focusList, model.focused)
}

func TestArchitectModel_Update_Selection(t *testing.T) {
	arch := &architecture.SystemArchitecture{
		Components: []architecture.Component{
			{ID: "comp-1", Type: "service"},
			{ID: "comp-2", Type: "worker"},
		},
	}
	model := initialModel(arch)

	// Simulate window resize to initialize viewport and dimensions
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 100, Height: 20})

	// Initial selection should be empty or first item if list auto-selects?
	// list.NewDefaultDelegate() selects first item by default?
	// Let's check model.selected after update.
	// We need to trigger list update to set selection.
	// Send a dummy key to list? No, Update calls list.Update which handles init.
	// But list.Model initializes selection to 0.
	// However, our `Update` logic sets `m.selected` based on `m.list.SelectedItem()`.

	// Send a Down key to list
	// Note: list needs to have focus
	model.focused = focusList

	// Check initial selection (after list update in WindowSizeMsg)
	// WindowSizeMsg calls list.Update, which should set selection if items > 0?
	// Actually list default selection is index 0.
	// But `m.selected` is nil initially.
	// In `Update`, we check `m.list.SelectedItem()` and set `m.selected`.
	// Since WindowSizeMsg calls `list.Update`, it should set `m.selected`.
	assert.NotNil(t, model.selected)
	assert.Equal(t, "comp-1", model.selected.ID)

	// Move down
	model, _ = updateModel(model, tea.KeyMsg{Type: tea.KeyDown})

	assert.Equal(t, "comp-2", model.selected.ID)
	assert.Contains(t, model.viewport.View(), "comp-2")
}

func TestArchitectModel_View(t *testing.T) {
	arch := &architecture.SystemArchitecture{
		Components: []architecture.Component{
			{ID: "comp-1", Type: "service", Description: "Desc 1"},
		},
	}
	model := initialModel(arch)

	// Initialize
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 100, Height: 20})

	view := model.View()
	assert.Contains(t, view, "comp-1")
	assert.Contains(t, view, "Desc 1")
	// Check for list border (default color)
	// Lipgloss uses ANSI codes, so hard to check exact string unless we strip ansi.
	// But checking content existence is good enough.
}

// Helper to cast tea.Model back to ArchitectModel
func updateModel(m ArchitectModel, msg tea.Msg) (ArchitectModel, tea.Cmd) {
	newM, cmd := m.Update(msg)
	return newM.(ArchitectModel), cmd
}
