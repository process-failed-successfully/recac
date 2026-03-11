package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func TestDashboard_SubmitState(t *testing.T) {
	m := NewDashboardModel("http://localhost:2112")
	m.viewState = viewMain

	// Press 's' to transition to viewSubmit
	m.table.SetWidth(100)
	m.table.SetHeight(20)

	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	model := newM.(DashboardModel)

	assert.Equal(t, viewSubmit, model.viewState, "Expected viewState to be viewSubmit after pressing 's'")
	assert.Equal(t, 0, model.focusedInput, "Expected first input to be focused")
	assert.True(t, model.inputs[0].Focused(), "Expected first textinput to be focused")

	// Type in the first input
	newM, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t', 'e', 's', 't'}})
	model = newM.(DashboardModel)
	assert.Equal(t, "test", model.inputs[0].Value(), "Expected first input to contain typed text")

	// Press Tab to focus next input
	newM, _ = model.Update(tea.KeyMsg{Type: tea.KeyTab})
	model = newM.(DashboardModel)
	assert.Equal(t, 1, model.focusedInput, "Expected second input to be focused after Tab")

	// Type in the second input
	newM, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r', 'e', 'p', 'o'}})
	model = newM.(DashboardModel)
	assert.Equal(t, "repo", model.inputs[1].Value(), "Expected second input to contain typed text")

	// Press Tab to focus third input (Depends On)
	newM, _ = model.Update(tea.KeyMsg{Type: tea.KeyTab})
	model = newM.(DashboardModel)
	assert.Equal(t, 2, model.focusedInput, "Expected third input to be focused after Tab")

	// Type in the third input
	newM, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'J', 'O', 'B', '-', '1'}})
	model = newM.(DashboardModel)
	assert.Equal(t, "JOB-1", model.inputs[2].Value(), "Expected third input to contain typed text")

	// Press Down to focus textarea
	newM, _ = model.Update(tea.KeyMsg{Type: tea.KeyDown})
	newM, _ = newM.(DashboardModel).Update(tea.KeyMsg{Type: tea.KeyDown})
	newM, _ = newM.(DashboardModel).Update(tea.KeyMsg{Type: tea.KeyDown})
	newM, _ = newM.(DashboardModel).Update(tea.KeyMsg{Type: tea.KeyDown})
	model = newM.(DashboardModel)
	assert.Equal(t, 6, model.focusedInput, "Expected textarea to be focused after Down")

	// Type in textarea
	newM, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d', 'e', 's', 'c'}})
	model = newM.(DashboardModel)
	assert.Equal(t, "desc", model.textarea.Value(), "Expected textarea to contain typed text")

	// Press Esc to cancel
	newM, _ = model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model = newM.(DashboardModel)
	assert.Equal(t, viewMain, model.viewState, "Expected viewState to be viewMain after pressing Esc")
}
