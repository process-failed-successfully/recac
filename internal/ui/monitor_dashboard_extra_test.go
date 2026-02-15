package ui

import (
	"errors"
	"recac/internal/model"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func TestMonitorDashboardModel_Update_ActionResult(t *testing.T) {
	m := NewMonitorDashboardModel(ActionCallbacks{})

	// Test Error Message
	errMsg := actionResultMsg{err: errors.New("test error")}
	newM, cmd := m.Update(errMsg)
	finalM := newM.(MonitorDashboardModel)

	assert.Contains(t, finalM.message, "Error: test error")
	assert.NotNil(t, cmd) // Should return tick cmd

	// Test Success Message
	successMsg := actionResultMsg{msg: "success"}
	newM, cmd = m.Update(successMsg)
	finalM = newM.(MonitorDashboardModel)

	assert.Equal(t, "success", finalM.message)
	assert.NotNil(t, cmd)

	// Test Clear Message (empty msg)
	clearMsg := actionResultMsg{msg: ""}
	newM, _ = m.Update(clearMsg)
	finalM = newM.(MonitorDashboardModel)

	assert.Equal(t, "", finalM.message)
}

func TestMonitorDashboardModel_View_Message(t *testing.T) {
	m := NewMonitorDashboardModel(ActionCallbacks{})
	m.message = "Test Message"
	view := m.View()
	assert.Contains(t, view, "Test Message")
}

func TestMonitorDashboardModel_Update_Tick(t *testing.T) {
	callbacks := ActionCallbacks{
		GetSessions: func() ([]model.UnifiedSession, error) {
			return []model.UnifiedSession{}, nil
		},
	}
	m := NewMonitorDashboardModel(callbacks)

	msg := monitorTickMsg(time.Now())
	_, cmd := m.Update(msg)

	// cmd should be a batch
	assert.NotNil(t, cmd)
}

func TestMonitorDashboardModel_Update_WindowSize(t *testing.T) {
	m := NewMonitorDashboardModel(ActionCallbacks{})
	msg := tea.WindowSizeMsg{Width: 100, Height: 50}
	newM, _ := m.Update(msg)
	finalM := newM.(MonitorDashboardModel)

	assert.Equal(t, 100, finalM.width)
	assert.Equal(t, 50, finalM.height)
	assert.Equal(t, 100, finalM.table.Width())
}
