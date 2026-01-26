package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func TestMenuModel_Selection(t *testing.T) {
	items := []MenuItem{
		{Name: "cmd1", Desc: "desc1"},
		{Name: "cmd2", Desc: "desc2"},
	}

	model := NewMenuModel(items)

	// Initial state
	assert.Equal(t, "", model.Selected)
	assert.False(t, model.Quitting)

	// Send Enter on first item
	updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})

	m := updatedModel.(MenuModel)
	assert.Equal(t, "cmd1", m.Selected)
	assert.False(t, m.Quitting)
}

func TestMenuModel_Navigation(t *testing.T) {
	items := []MenuItem{
		{Name: "cmd1", Desc: "desc1"},
		{Name: "cmd2", Desc: "desc2"},
	}

	model := NewMenuModel(items)

	// Move down
	updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyDown})

	// Select
	updatedModel, _ = updatedModel.Update(tea.KeyMsg{Type: tea.KeyEnter})

	m := updatedModel.(MenuModel)
	assert.Equal(t, "cmd2", m.Selected)
}

func TestMenuModel_Quit(t *testing.T) {
	items := []MenuItem{{Name: "cmd1"}}
	model := NewMenuModel(items)

	// Send Ctrl+C
	updatedModel, cmd := model.Update(tea.KeyMsg{Type: tea.KeyCtrlC})

	m := updatedModel.(MenuModel)
	assert.True(t, m.Quitting)
	// tea.Quit returns a command, verifying it is returned
	assert.NotNil(t, cmd)
}
