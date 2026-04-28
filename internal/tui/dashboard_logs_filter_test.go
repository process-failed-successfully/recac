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

func TestDashboardModel_UpdateLogsView(t *testing.T) {
	m := NewDashboardModel("localhost:8080")
	m.viewState = viewLogs
	m.isLogFiltering = true

	// Simulate Esc key
	mEsc, _ := m.updateLogsView(tea.KeyMsg{Type: tea.KeyEsc})

	assert.False(t, mEsc.isLogFiltering)
	assert.Empty(t, mEsc.logFilterInput.Value())

	// Simulate Enter key
	m.isLogFiltering = true
	m.logFilterInput.SetValue("test")
	mEnter, _ := m.updateLogsView(tea.KeyMsg{Type: tea.KeyEnter})

	assert.False(t, mEnter.isLogFiltering)
	assert.Equal(t, "test", mEnter.logFilterInput.Value())

	// Simulate normal typing
	m.isLogFiltering = true
	m.logFilterInput.SetValue("t")
	mType, _ := m.updateLogsView(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}, Alt: false})
	assert.True(t, mType.isLogFiltering)
}

func TestDashboardModel_UpdateLogsView_Normal(t *testing.T) {
	m := NewDashboardModel("localhost:8080")
	m.viewState = viewLogs
	m.isLogFiltering = false

	// Test slash to start filtering
	// Key mapping for slash is runes '/' but String() will return "/"
	mSlash, _ := m.updateLogsView(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})

	assert.True(t, mSlash.isLogFiltering)

	// Test esc to clear filter or quit when filter is already empty
	mEscEmpty, _ := m.updateLogsView(tea.KeyMsg{Type: tea.KeyEsc})
	assert.Equal(t, viewMain, mEscEmpty.viewState)
	assert.False(t, mEscEmpty.isLogFiltering)

	// Test esc to clear filter when filter HAS value
	mSlashWithVal := mSlash
	mSlashWithVal.logFilterInput.SetValue("query")

	mEscWithVal, _ := mSlashWithVal.updateLogsView(tea.KeyMsg{Type: tea.KeyEsc})

	assert.Equal(t, viewLogs, mEscWithVal.viewState) // Check if it keeps it in logs view
	assert.Empty(t, mEscWithVal.logFilterInput.Value())
}

func TestDashboardModel_UpdateLogsView_ExtraKeys(t *testing.T) {
	m := NewDashboardModel("localhost:8080")
	m.viewState = viewLogs
	m.isLogFiltering = false

	// Try a key that does not trigger anything in the log switch (e.g. key 'j')
	mAny, cmdAny := m.updateLogsView(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	assert.False(t, mAny.isLogFiltering)
	_ = cmdAny // viewport update command
}
