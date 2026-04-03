package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func TestDashboardLogFiltering(t *testing.T) {
	m := NewDashboardModel("http://dummy")

	// Simulate opening logs view
	m.viewState = viewLogs
	m.logs = "INFO: starting job\nDEBUG: running task\nERROR: something went wrong\nINFO: finishing job"

	// Press '/' to start filtering
	mModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	m = mModel.(DashboardModel)

	assert.True(t, m.isLogFiltering)
	assert.NotNil(t, cmd) // textinput.Blink

	// Type 'error'
	for _, r := range []rune("error") {
		mModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = mModel.(DashboardModel)
	}

	assert.Equal(t, "error", m.logFilterInput.Value())

	// The viewport should only contain the filtered line
	content := m.viewport.View()
	assert.Contains(t, content, "ERROR: something went wrong")
	assert.NotContains(t, content, "INFO: starting job")
	assert.NotContains(t, content, "DEBUG: running task")

	// Press 'esc' to cancel filtering
	mModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = mModel.(DashboardModel)

	assert.False(t, m.isLogFiltering)
	assert.Equal(t, "", m.logFilterInput.Value())

	// The viewport should contain all logs again
	content = m.viewport.View()
	assert.Contains(t, content, "INFO: starting job")
	assert.Contains(t, content, "ERROR: something went wrong")

	// Simulate incoming chunk while filtering
	m.isLogFiltering = true
	m.logFilterInput.SetValue("warning")
	m.logs = "WARN: old warning"
	m.updateFilteredLogs()

	// Send new log chunk
	mModel, _ = m.Update(logChunkMsg{Chunk: "\nERROR: fail\nWARNING: new warning"})
	m = mModel.(DashboardModel)

	content = m.viewport.View()
	assert.Contains(t, content, "WARN: old warning")
	assert.Contains(t, content, "WARNING: new warning")
	assert.NotContains(t, content, "ERROR: fail")
}
