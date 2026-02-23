package tui

import (
	"fmt"
	"io"
	"recac/internal/orchestrator"
	"strings"
	"testing"
	"time"

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

func TestDashboardModel_Update_StatusMsg(t *testing.T) {
	model := NewDashboardModel("http://localhost")
	now := time.Now()
	msg := statusMsg{
		Status: orchestrator.Status{Uptime: "1h"},
		Jobs: []orchestrator.JobInfo{
			{ID: "job-1", Summary: "Test", Status: "running", StartTime: now},
		},
	}

	updatedModel, cmd := model.Update(msg)
	m, ok := updatedModel.(DashboardModel)
	assert.True(t, ok)
	assert.Nil(t, cmd)
	assert.Equal(t, "1h", m.status.Uptime)
	assert.Len(t, m.jobs, 1)

	// Check table content
	rows := m.table.Rows()
	assert.Len(t, rows, 1)
	assert.Equal(t, "job-1", rows[0][0])
}
