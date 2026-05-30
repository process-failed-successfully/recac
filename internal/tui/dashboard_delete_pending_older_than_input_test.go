package tui

import (
	"testing"
	"github.com/stretchr/testify/assert"
	tea "github.com/charmbracelet/bubbletea"
)

func TestUpdateDeletePendingOlderThanInput(t *testing.T) {
	m := NewDashboardModel("http://localhost")
	m.viewState = viewDeletePendingOlderThanInput

	// Test Esc key
	m, cmd := m.updateDeletePendingOlderThanInput(tea.KeyMsg{Type: tea.KeyEsc})
	assert.Nil(t, cmd)
	assert.Equal(t, viewMain, m.viewState)

	// Reset state
	m.viewState = viewDeletePendingOlderThanInput
	m.deletePendingOlderThanInput.Focus()

	// Type something
	m, cmd = m.updateDeletePendingOlderThanInput(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2', '4', 'h'}})
	assert.NotNil(t, cmd) // cursor blink cmd
	assert.Equal(t, "24h", m.deletePendingOlderThanInput.Value())

	// Test Enter with value
	m, cmd = m.updateDeletePendingOlderThanInput(tea.KeyMsg{Type: tea.KeyEnter})
	assert.NotNil(t, cmd) // should return deletePendingBulkCmd
	assert.Equal(t, viewMain, m.viewState)

	// Reset state and test empty Enter
	m.viewState = viewDeletePendingOlderThanInput
	m.deletePendingOlderThanInput.SetValue("")
	m.deletePendingOlderThanInput.Focus()

	m, cmd = m.updateDeletePendingOlderThanInput(tea.KeyMsg{Type: tea.KeyEnter})
	assert.Nil(t, cmd)
	assert.Equal(t, viewMain, m.viewState)
	assert.NotNil(t, m.err)
	assert.Contains(t, m.err.Error(), "cannot be empty")
}
