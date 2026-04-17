package tui

import (
	"fmt"
	"io"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func TestDashboardModel_Update_LogFinishedMsg(t *testing.T) {
	model := NewDashboardModel("http://localhost")
	model.logStream = io.NopCloser(strings.NewReader(""))

	msg := logFinishedMsg{}
	updatedModel, _ := model.Update(msg)
	m, ok := updatedModel.(DashboardModel)
	assert.True(t, ok)
	assert.Nil(t, m.logStream)

	// Error
	errMsg := logFinishedMsg{Err: fmt.Errorf("finished error")}
	updatedModel, _ = model.Update(errMsg)
	m, ok = updatedModel.(DashboardModel)
	assert.True(t, ok)
	assert.NotNil(t, m.err)
}

func TestDashboardModel_Update_ActionMsg(t *testing.T) {
	model := NewDashboardModel("http://localhost")

	// Success
	msg := actionMsg{Message: "Success"}
	updatedModel, cmd := model.Update(msg)
	_, ok := updatedModel.(DashboardModel)
	assert.True(t, ok)
	assert.NotNil(t, cmd) // Should refresh status

	// Error
	errMsg := actionMsg{Err: fmt.Errorf("action error")}
	updatedModel, cmd = model.Update(errMsg)
	m, ok := updatedModel.(DashboardModel)
	assert.True(t, ok)
	assert.NotNil(t, m.err)
}

func TestDashboardModel_Update_WindowSize(t *testing.T) {
	model := NewDashboardModel("http://localhost")
	msg := tea.WindowSizeMsg{Width: 100, Height: 50}

	updatedModel, _ := model.Update(msg)
	m, ok := updatedModel.(DashboardModel)
	assert.True(t, ok)
	assert.Equal(t, 100, m.viewport.Width)
	assert.Equal(t, 45, m.viewport.Height) // 50 - 5
}

func TestDashboardModel_UpdateViewport(t *testing.T) {
	model := NewDashboardModel("http://localhost")
	model.viewState = viewExplain // set to some other state
	model.logStream = io.NopCloser(strings.NewReader("some logs"))

	// Test quit / esc key
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}
	m, cmd := model.updateViewport(msg)

	assert.Equal(t, viewMain, m.viewState)
	assert.Nil(t, m.logStream)
	assert.Nil(t, cmd)

	// Test esc key
	model.viewState = viewExplain
	msgEsc := tea.KeyMsg{Type: tea.KeyEsc}
	m2, cmd2 := model.updateViewport(msgEsc)

	assert.Equal(t, viewMain, m2.viewState)
	assert.Nil(t, m2.logStream)
	assert.Nil(t, cmd2)

	// Test other key, should go to viewport
	model.viewState = viewExplain
	msgDown := tea.KeyMsg{Type: tea.KeyDown}
	m3, _ := model.updateViewport(msgDown)

	assert.Equal(t, viewExplain, m3.viewState) // shouldn't change
}
