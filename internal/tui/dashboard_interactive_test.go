package tui

import (
	"recac/internal/orchestrator"
	"testing"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func TestDashboardModel_Update_EnterDetails(t *testing.T) {
	// Setup
	columns := []table.Column{{Title: "ID", Width: 10}}
	tModel := table.New(table.WithColumns(columns))
	rows := []table.Row{{"JOB-1"}}
	tModel.SetRows(rows)
	tModel.SetCursor(0)

	model := DashboardModel{
		host:      "http://localhost",
		table:     tModel,
		viewState: viewMain,
	}

	// Act: Press Enter
	updatedModel, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m := updatedModel.(DashboardModel)

	// Assert: Command should be returned (fetchDetails)
	assert.NotNil(t, cmd)
	// View state shouldn't change yet (until msg received)
	assert.Equal(t, viewMain, m.viewState)
}

func TestDashboardModel_Update_DetailsMsg(t *testing.T) {
	vp := viewport.New(100, 20)
	model := DashboardModel{
		viewState: viewMain,
		viewport:  vp,
	}
	job := orchestrator.JobInfo{ID: "JOB-1", Summary: "Test"}
	msg := detailsMsg{Job: job}

	updatedModel, _ := model.Update(msg)
	m := updatedModel.(DashboardModel)

	assert.Equal(t, viewDetails, m.viewState)
	assert.Equal(t, job, m.details)
}

func TestDashboardModel_Update_BackFromDetails(t *testing.T) {
	model := DashboardModel{
		viewState: viewDetails,
	}

	updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m := updatedModel.(DashboardModel)

	assert.Equal(t, viewMain, m.viewState)
}

func TestDashboardModel_Update_ToggleHistory(t *testing.T) {
	model := DashboardModel{
		viewState:   viewMain,
		showHistory: false,
	}

	updatedModel, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("h")})
	m := updatedModel.(DashboardModel)

	assert.True(t, m.showHistory)
	assert.NotNil(t, cmd) // Should fetch status
}

func TestDashboardModel_Update_LogsMsg(t *testing.T) {
	vp := viewport.New(100, 20)
	model := DashboardModel{
		viewState: viewMain,
		viewport:  vp,
	}
	logs := "some logs"
	msg := logsMsg{Logs: logs}

	updatedModel, _ := model.Update(msg)
	m := updatedModel.(DashboardModel)

	assert.Equal(t, viewLogs, m.viewState)
	assert.Equal(t, logs, m.logs)
}
