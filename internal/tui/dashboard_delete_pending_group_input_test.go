package tui

import (
	"testing"
	"github.com/stretchr/testify/assert"
	tea "github.com/charmbracelet/bubbletea"
)

func TestUpdateDeletePendingGroupInput(t *testing.T) {
	m := NewDashboardModel("http://localhost")
	m.viewState = viewDeletePendingGroupInput

	// Test Esc key
	m, cmd := m.updateDeletePendingGroupInput(tea.KeyMsg{Type: tea.KeyEsc})
	assert.Nil(t, cmd)
	assert.Equal(t, viewMain, m.viewState)

	// Reset state
	m.viewState = viewDeletePendingGroupInput
	m.deletePendingGroupInput.Focus()

	// Type something
	m, cmd = m.updateDeletePendingGroupInput(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g', '1'}})
	assert.NotNil(t, cmd) // cursor blink cmd
	assert.Equal(t, "g1", m.deletePendingGroupInput.Value())

	// Test Enter with value
	m, cmd = m.updateDeletePendingGroupInput(tea.KeyMsg{Type: tea.KeyEnter})
	assert.NotNil(t, cmd) // should return deletePendingBulkCmd
	assert.Equal(t, viewMain, m.viewState)

	// Reset state and test empty Enter
	m.viewState = viewDeletePendingGroupInput
	m.deletePendingGroupInput.SetValue("")
	m.deletePendingGroupInput.Focus()

	m, cmd = m.updateDeletePendingGroupInput(tea.KeyMsg{Type: tea.KeyEnter})
	assert.Nil(t, cmd)
	assert.Equal(t, viewMain, m.viewState)
	assert.NotNil(t, m.err)
	assert.Contains(t, m.err.Error(), "cannot be empty")
}
