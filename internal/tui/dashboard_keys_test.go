package tui

import (
	"fmt"
	"recac/internal/orchestrator"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func TestDashboardModel_Keys(t *testing.T) {
	columns := []table.Column{{Title: "ID", Width: 10}}
	tModel := table.New(table.WithColumns(columns))
	rows := []table.Row{{"JOB-1"}}
	tModel.SetRows(rows)
	tModel.SetCursor(0)

	model := DashboardModel{
		host:      "http://localhost",
		table:     tModel,
		viewState: viewMain,
		jobs: []orchestrator.JobInfo{
			{ID: "JOB-1", StartTime: time.Now()},
		},
	}

	t.Run("Logs Key (l)", func(t *testing.T) {
		updatedModel, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
		_, ok := updatedModel.(DashboardModel)
		assert.True(t, ok)
		assert.NotNil(t, cmd) // Should return fetchJobLogs
	})

	t.Run("Cancel Key (c)", func(t *testing.T) {
		// Reset state
		model.viewState = viewMain
		updatedModel, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
		m, ok := updatedModel.(DashboardModel)
		assert.True(t, ok)
		assert.Nil(t, cmd) // Should return nil, waiting for confirmation
		assert.Equal(t, viewConfirmation, m.viewState)
		assert.Equal(t, "cancel", m.pendingAction)
	})

	t.Run("Cancel All Key (C)", func(t *testing.T) {
		// Reset state
		model.viewState = viewMain
		updatedModel, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("C")})
		m, ok := updatedModel.(DashboardModel)
		assert.True(t, ok)
		assert.Nil(t, cmd) // Should return nil, waiting for confirmation
		assert.Equal(t, viewConfirmation, m.viewState)
		assert.Equal(t, "cancel all", m.pendingAction)
		assert.Equal(t, "ALL", m.pendingJobId)
	})

	t.Run("Retry Key (r)", func(t *testing.T) {
		// Reset state
		model.viewState = viewMain
		updatedModel, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
		m, ok := updatedModel.(DashboardModel)
		assert.True(t, ok)
		assert.Nil(t, cmd) // Should return nil, waiting for confirmation
		assert.Equal(t, viewConfirmation, m.viewState)
		assert.Equal(t, "retry", m.pendingAction)
	})

	t.Run("Retry Failed Key (R)", func(t *testing.T) {
		// Reset state
		model.viewState = viewMain
		updatedModel, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("R")})
		m, ok := updatedModel.(DashboardModel)
		assert.True(t, ok)
		assert.Nil(t, cmd) // Should return nil, waiting for confirmation
		assert.Equal(t, viewConfirmation, m.viewState)
		assert.Equal(t, "retry failed", m.pendingAction)
		assert.Equal(t, "FAILED", m.pendingJobId)
	})

	t.Run("Quit Key (q)", func(t *testing.T) {
		updatedModel, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
		m, ok := updatedModel.(DashboardModel)
		assert.True(t, ok)
		assert.True(t, m.quitting)
		assert.Equal(t, tea.Quit(), cmd())
	})
}

func TestDashboardModel_View_States(t *testing.T) {
	model := DashboardModel{
		host: "host",
		status: orchestrator.Status{
			Uptime: "1h",
		},
	}

	t.Run("View Details", func(t *testing.T) {
		model.viewState = viewDetails
		view := model.View()
		assert.Contains(t, view, "esc/q: back")
	})

	t.Run("View Logs", func(t *testing.T) {
		model.viewState = viewLogs
		view := model.View()
		assert.Contains(t, view, "esc/q: back")
	})

	t.Run("View Error", func(t *testing.T) {
		model.viewState = viewMain
		model.err = fmt.Errorf("some error")
		view := model.View()
		assert.Contains(t, view, "Error: some error")
	})
}
