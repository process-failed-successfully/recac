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

	t.Run("Open Repo Key (o)", func(t *testing.T) {
		// Store original util function and restore it later to mock browser opening
		originalOpenBrowser := utilsOpenBrowser
		defer func() { utilsOpenBrowser = originalOpenBrowser }()

		browserOpened := false
		openedUrl := ""
		utilsOpenBrowser = func(url string) error {
			browserOpened = true
			openedUrl = url
			return nil
		}

		// Ensure job has a RepoURL
		if len(model.jobs) > 0 {
			model.jobs[0].WorkItem.RepoURL = "https://github.com/org/test-repo"
		}

		updatedModel, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("o")})
		_, ok := updatedModel.(DashboardModel)
		assert.True(t, ok)
		assert.NotNil(t, cmd) // Should return openBrowserCmd

		msg := cmd()
		action, isAction := msg.(actionMsg)
		assert.True(t, isAction)
		assert.Equal(t, "Opened browser", action.Message)
		assert.True(t, browserOpened)
		assert.Equal(t, "https://github.com/org/test-repo", openedUrl)
	})

	t.Run("Open Repo Key (o) - No URL", func(t *testing.T) {
		// Ensure job has no RepoURL
		if len(model.jobs) > 0 {
			model.jobs[0].WorkItem.RepoURL = ""
		}

		updatedModel, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("o")})
		_, ok := updatedModel.(DashboardModel)
		assert.True(t, ok)
		assert.NotNil(t, cmd)

		msg := cmd()
		action, isAction := msg.(actionMsg)
		assert.True(t, isAction)
		assert.Error(t, action.Err)
		assert.Contains(t, action.Err.Error(), "no repo url")
	})

	t.Run("Logs Key (l)", func(t *testing.T) {
		updatedModel, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
		_, ok := updatedModel.(DashboardModel)
		assert.True(t, ok)
		assert.NotNil(t, cmd) // Should return fetchJobLogs
	})

	t.Run("Pause Key (p)", func(t *testing.T) {
		updatedModel, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("p")})
		_, ok := updatedModel.(DashboardModel)
		assert.True(t, ok)
		assert.NotNil(t, cmd) // Should return togglePause
	})

	t.Run("Force Poll Key (f)", func(t *testing.T) {
		updatedModel, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("f")})
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

	t.Run("Clear Pending Key (P)", func(t *testing.T) {
		// Reset state
		model.viewState = viewMain
		updatedModel, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("P")})
		m, ok := updatedModel.(DashboardModel)
		assert.True(t, ok)
		assert.Nil(t, cmd) // Should return nil, waiting for confirmation
		assert.Equal(t, viewConfirmation, m.viewState)
		assert.Equal(t, "clear pending", m.pendingAction)
		assert.Equal(t, "PENDING", m.pendingJobId)
	})

	t.Run("Edit/Clone Key (e)", func(t *testing.T) {
		// Ensure job has details
		if len(model.jobs) > 0 {
			model.jobs[0].Summary = "Test Summary"
			model.jobs[0].WorkItem.RepoURL = "https://github.com/org/test"
			model.jobs[0].WorkItem.DependsOn = []string{"dep-1", "dep-2"}
			model.jobs[0].WorkItem.Description = "Test Description"
		}

		// Initialize inputs and textarea if not already present
		if len(model.inputs) == 0 {
			model = NewDashboardModel("http://localhost")
			model.table = tModel
			model.jobs = []orchestrator.JobInfo{
				{
					ID:        "JOB-1",
					StartTime: time.Now(),
					Summary:   "Test Summary",
					WorkItem: orchestrator.WorkItem{
						RepoURL:     "https://github.com/org/test",
						DependsOn:   []string{"dep-1", "dep-2"},
						Description: "Test Description",
					},
				},
			}
		}

		model.viewState = viewMain
		updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("e")})
		m, ok := updatedModel.(DashboardModel)
		assert.True(t, ok)
		assert.Equal(t, viewSubmit, m.viewState)

		// Verify fields are pre-filled
		assert.Equal(t, "Test Summary", m.inputs[0].Value())
		assert.Equal(t, "https://github.com/org/test", m.inputs[1].Value())
		assert.Equal(t, "dep-1,dep-2", m.inputs[2].Value())
		assert.Equal(t, "Test Description", m.textarea.Value())
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
