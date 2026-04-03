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

	t.Run("Copy to Clipboard Key (y)", func(t *testing.T) {
		originalClipboard := clipboardWriteAll
		defer func() { clipboardWriteAll = originalClipboard }()

		var copiedText string
		clipboardWriteAll = func(text string) error {
			copiedText = text
			return nil
		}

		updatedModel, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
		_, ok := updatedModel.(DashboardModel)
		assert.True(t, ok)
		assert.NotNil(t, cmd)

		msg := cmd()
		action, isAction := msg.(actionMsg)
		assert.True(t, isAction)
		assert.Equal(t, "Copied 1 job ID(s) to clipboard", action.Message)
		assert.Equal(t, "JOB-1", copiedText)
	})

	t.Run("Copy to Clipboard Key (y) - Multiple", func(t *testing.T) {
		originalClipboard := clipboardWriteAll
		defer func() { clipboardWriteAll = originalClipboard }()

		var copiedText string
		clipboardWriteAll = func(text string) error {
			copiedText = text
			return nil
		}

		m := model
		m.selectedJobs = map[string]bool{"JOB-1": true, "JOB-2": true}

		updatedModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
		_, ok := updatedModel.(DashboardModel)
		assert.True(t, ok)
		assert.NotNil(t, cmd)

		msg := cmd()
		action, isAction := msg.(actionMsg)
		assert.True(t, isAction)
		assert.Equal(t, "Copied 2 job ID(s) to clipboard", action.Message)
		assert.Equal(t, "JOB-1,JOB-2", copiedText) // Because of sort.Strings
	})

	t.Run("Copy to Clipboard Key (y) - Error", func(t *testing.T) {
		originalClipboard := clipboardWriteAll
		defer func() { clipboardWriteAll = originalClipboard }()

		clipboardWriteAll = func(text string) error {
			return fmt.Errorf("clipboard error")
		}

		updatedModel, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
		_, ok := updatedModel.(DashboardModel)
		assert.True(t, ok)
		assert.NotNil(t, cmd)

		msg := cmd()
		action, isAction := msg.(actionMsg)
		assert.True(t, isAction)
		assert.Error(t, action.Err)
		assert.Contains(t, action.Err.Error(), "clipboard error")
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

	t.Run("Cancel Downstream Key (ctrl+x)", func(t *testing.T) {
		// Reset state
		model.viewState = viewMain
		updatedModel, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("ctrl+x")})
		m, ok := updatedModel.(DashboardModel)
		assert.True(t, ok)
		assert.Nil(t, cmd) // Should return nil, waiting for confirmation
		assert.Equal(t, viewConfirmation, m.viewState)
		assert.Equal(t, "cancel downstream", m.pendingAction)
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

	t.Run("Hold Key (H)", func(t *testing.T) {
		// Reset state
		model.viewState = viewMain
		updatedModel, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("H")})
		m, ok := updatedModel.(DashboardModel)
		assert.True(t, ok)
		assert.Nil(t, cmd) // Should return nil, waiting for confirmation
		assert.Equal(t, viewConfirmation, m.viewState)
		assert.Equal(t, "hold", m.pendingAction)
		assert.Equal(t, "JOB-1", m.pendingJobId)
	})

	t.Run("Unhold Key (U)", func(t *testing.T) {
		// Reset state
		model.viewState = viewMain
		updatedModel, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("U")})
		m, ok := updatedModel.(DashboardModel)
		assert.True(t, ok)
		assert.Nil(t, cmd) // Should return nil, waiting for confirmation
		assert.Equal(t, viewConfirmation, m.viewState)
		assert.Equal(t, "unhold", m.pendingAction)
		assert.Equal(t, "JOB-1", m.pendingJobId)
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

	t.Run("Retry Downstream Key (ctrl+y)", func(t *testing.T) {
		// Reset state
		model.viewState = viewMain
		updatedModel, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("ctrl+y")})
		m, ok := updatedModel.(DashboardModel)
		assert.True(t, ok)
		assert.Nil(t, cmd) // Should return nil, waiting for confirmation
		assert.Equal(t, viewConfirmation, m.viewState)
		assert.Equal(t, "retry downstream", m.pendingAction)
	})

	t.Run("Update Env", func(t *testing.T) {
		m := NewDashboardModel("http://localhost:2112")
		m.jobs = []orchestrator.JobInfo{
			{ID: "job-1", WorkItem: orchestrator.WorkItem{EnvVars: map[string]string{"K": "V"}}},
		}
		m.updateTableContent()
		m.table.SetCursor(0)

		// Send 'E'
		newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("E"), Alt: false})
		m = newModel.(DashboardModel)

		// Verify view state changes
		assert.Equal(t, viewEnvInput, m.viewState)
		assert.Equal(t, "job-1", m.pendingJobId)
		assert.Equal(t, "K=V", m.envInput.Value())

		// Test Enter to confirm env
		m.envInput.SetValue("A=B, C=D")
		newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("enter"), Alt: false})
		m = newModel.(DashboardModel)

		assert.NotNil(t, cmd) // Should return the updateEnvCmd
		assert.Equal(t, viewMain, m.viewState)
		assert.Equal(t, "", m.pendingJobId)
	})

	t.Run("Update Tags", func(t *testing.T) {
		m := NewDashboardModel("http://localhost:2112")
		m.jobs = []orchestrator.JobInfo{
			{ID: "job-1", WorkItem: orchestrator.WorkItem{Tags: []string{"t1", "t2"}}},
		}
		m.updateTableContent()
		m.table.SetCursor(0)

		// Send 'G'
		newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("G"), Alt: false})
		m = newModel.(DashboardModel)

		// Verify view state changes
		assert.Equal(t, viewTagsInput, m.viewState)
		assert.Equal(t, "job-1", m.pendingJobId)
		assert.Equal(t, "t1, t2", m.tagsInput.Value())

		// Test Enter to confirm tags
		m.tagsInput.SetValue("new-t1, new-t2")
		newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("enter"), Alt: false})
		m = newModel.(DashboardModel)

		assert.NotNil(t, cmd) // Should return the updateTagsCmd
		assert.Equal(t, viewMain, m.viewState)
		assert.Equal(t, "", m.pendingJobId)
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

	t.Run("Delete Pending Key (backspace)", func(t *testing.T) {
		// Reset state
		model.viewState = viewMain
		model.selectedJobs = make(map[string]bool)
		// For special keys, we construct them:
		msg := tea.KeyMsg{Type: tea.KeyBackspace}
		updatedModel, cmd := model.Update(msg)
		m, ok := updatedModel.(DashboardModel)
		assert.True(t, ok)
		assert.Nil(t, cmd) // Should return nil, waiting for confirmation
		assert.Equal(t, viewConfirmation, m.viewState)
		assert.Equal(t, "delete pending", m.pendingAction)
		assert.Equal(t, "JOB-1", m.pendingJobId)
	})

	t.Run("Delete Pending Key (delete)", func(t *testing.T) {
		// Reset state
		model.viewState = viewMain
		model.selectedJobs = make(map[string]bool)
		msg := tea.KeyMsg{Type: tea.KeyDelete}
		updatedModel, cmd := model.Update(msg)
		m, ok := updatedModel.(DashboardModel)
		assert.True(t, ok)
		assert.Nil(t, cmd) // Should return nil, waiting for confirmation
		assert.Equal(t, viewConfirmation, m.viewState)
		assert.Equal(t, "delete pending", m.pendingAction)
		assert.Equal(t, "JOB-1", m.pendingJobId)
	})

	t.Run("Delete Pending Multiple Key (delete)", func(t *testing.T) {
		// Reset state
		model.viewState = viewMain
		model.selectedJobs = map[string]bool{"JOB-1": true}
		msg := tea.KeyMsg{Type: tea.KeyDelete}
		updatedModel, cmd := model.Update(msg)
		m, ok := updatedModel.(DashboardModel)
		assert.True(t, ok)
		assert.Nil(t, cmd) // Should return nil, waiting for confirmation
		assert.Equal(t, viewConfirmation, m.viewState)
		assert.Equal(t, "delete pending multiple", m.pendingAction)
		assert.Equal(t, "MULTIPLE_delete_pending", m.pendingJobId)
	})

	t.Run("Set Deps Key (D)", func(t *testing.T) {
		// Ensure job has details
		if len(model.jobs) > 0 {
			model.jobs[0].WorkItem.DependsOn = []string{"dep-1", "dep-2"}
		}

		// Re-initialize model to get a fresh depsInput component
		model = NewDashboardModel("http://localhost")
		model.table = tModel
		model.jobs = []orchestrator.JobInfo{
			{
				ID:        "JOB-1",
				StartTime: time.Now(),
				WorkItem: orchestrator.WorkItem{
					DependsOn: []string{"dep-1", "dep-2"},
				},
			},
		}

		model.viewState = viewMain
		updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("D")})
		m, ok := updatedModel.(DashboardModel)
		assert.True(t, ok)
		assert.Equal(t, viewDepsInput, m.viewState)

		// Verify fields are pre-filled
		assert.Equal(t, "dep-1, dep-2", m.depsInput.Value())
	})

	t.Run("Set Deps Key (D) - Multiple", func(t *testing.T) {
		model = NewDashboardModel("http://localhost")
		model.table = tModel
		model.jobs = []orchestrator.JobInfo{
			{ID: "JOB-1"},
		}
		model.selectedJobs = map[string]bool{"JOB-1": true}
		model.viewState = viewMain
		updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("D")})
		m, ok := updatedModel.(DashboardModel)
		assert.True(t, ok)
		assert.Equal(t, viewDepsInput, m.viewState)
		assert.Equal(t, "MULTIPLE_deps", m.pendingJobId)
		assert.Equal(t, "", m.depsInput.Value())
	})

	t.Run("Update Env (E) - Multiple", func(t *testing.T) {
		model = NewDashboardModel("http://localhost")
		model.table = tModel
		model.jobs = []orchestrator.JobInfo{
			{ID: "JOB-1"},
		}
		model.selectedJobs = map[string]bool{"JOB-1": true}
		model.viewState = viewMain
		updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("E")})
		m, ok := updatedModel.(DashboardModel)
		assert.True(t, ok)
		assert.Equal(t, viewEnvInput, m.viewState)
		assert.Equal(t, "MULTIPLE_env", m.pendingJobId)
		assert.Equal(t, "", m.envInput.Value())
	})

	t.Run("View Tags Key (L)", func(t *testing.T) {
		model = NewDashboardModel("http://localhost")
		model.viewState = viewMain
		updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("L")})
		m, ok := updatedModel.(DashboardModel)
		assert.True(t, ok)
		// We can only test that it attempts to fetch tags since we aren't mocking the HTTP layer here directly
		// It returns a fetchTagsCmd tea.Cmd. We just assert the view didn't change unexpectedly and state is intact.
		assert.Equal(t, viewMain, m.viewState)
	})

	t.Run("Update Tags (G) - Multiple", func(t *testing.T) {
		model = NewDashboardModel("http://localhost")
		model.table = tModel
		model.jobs = []orchestrator.JobInfo{
			{ID: "JOB-1"},
		}
		model.selectedJobs = map[string]bool{"JOB-1": true}
		model.viewState = viewMain
		updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("G")})
		m, ok := updatedModel.(DashboardModel)
		assert.True(t, ok)
		assert.Equal(t, viewTagsInput, m.viewState)
		assert.Equal(t, "MULTIPLE_tags", m.pendingJobId)
		assert.Equal(t, "", m.tagsInput.Value())
	})

	t.Run("Update Agent (M)", func(t *testing.T) {
		m := NewDashboardModel("http://localhost:2112")
		m.jobs = []orchestrator.JobInfo{
			{ID: "job-1", WorkItem: orchestrator.WorkItem{AgentProvider: "openrouter", AgentModel: "openai/gpt-4o"}},
		}
		m.updateTableContent()
		m.table.SetCursor(0)

		// Send 'M'
		newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("M"), Alt: false})
		m = newModel.(DashboardModel)

		// Verify view state changes
		assert.Equal(t, viewAgentInput, m.viewState)
		assert.Equal(t, "job-1", m.pendingJobId)
		assert.Equal(t, "openrouter", m.agentProviderInput.Value())
		assert.Equal(t, "openai/gpt-4o", m.agentModelInput.Value())

		// Test Enter to confirm agent
		m.agentProviderInput.SetValue("anthropic")
		m.agentModelInput.SetValue("claude-3-5-sonnet")
		newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("enter"), Alt: false})
		m = newModel.(DashboardModel)

		assert.NotNil(t, cmd) // Should return the updateAgentCmd
		assert.Equal(t, viewMain, m.viewState)
		assert.Equal(t, "", m.pendingJobId)
	})

	t.Run("Update Agent (M) - Multiple", func(t *testing.T) {
		model = NewDashboardModel("http://localhost")
		model.table = tModel
		model.jobs = []orchestrator.JobInfo{
			{ID: "JOB-1"},
		}
		model.selectedJobs = map[string]bool{"JOB-1": true}
		model.viewState = viewMain
		updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("M")})
		m, ok := updatedModel.(DashboardModel)
		assert.True(t, ok)
		assert.Equal(t, viewAgentInput, m.viewState)
		assert.Equal(t, "MULTIPLE_agent", m.pendingJobId)
		assert.Equal(t, "", m.agentProviderInput.Value())
		assert.Equal(t, "", m.agentModelInput.Value())
	})

	t.Run("Edit/Clone Key (e)", func(t *testing.T) {
		maxRet := 5
		// Ensure job has details
		if len(model.jobs) > 0 {
			model.jobs[0].Summary = "Test Summary"
			model.jobs[0].WorkItem.RepoURL = "https://github.com/org/test"
			model.jobs[0].WorkItem.DependsOn = []string{"dep-1", "dep-2"}
			model.jobs[0].WorkItem.Description = "Test Description"
			model.jobs[0].WorkItem.MaxRetries = &maxRet
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
						MaxRetries:  &maxRet,
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
		assert.Equal(t, "5", m.inputs[8].Value())
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

	t.Run("Rename Key (N)", func(t *testing.T) {
		m := NewDashboardModel("http://localhost:2112")
		m.jobs = []orchestrator.JobInfo{
			{ID: "JOB-1", Summary: "Test Job", Status: "Pending"},
		}
		m.updateTableContent()
		m.table.SetCursor(0)

		updatedModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("N")})
		m = updatedModel.(DashboardModel)

		assert.Equal(t, viewRenameInput, m.viewState)
		assert.Equal(t, "JOB-1", m.pendingJobId)
		assert.Equal(t, "JOB-1", m.renameInput.Value())
		assert.NotNil(t, cmd) // Should return textinput.Blink
	})
}

func TestDashboardModel_UpdateSubmit(t *testing.T) {
	mModel := NewDashboardModel("http://localhost:8080")
	mModel.viewState = viewSubmit

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("1")}
	newModel, _ := mModel.updateSubmit(msg)
	assert.NotNil(t, newModel)

	msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("2")}
	newModel, _ = newModel.updateSubmit(msg)
	assert.NotNil(t, newModel)

    msg = tea.KeyMsg{Type: tea.KeyEsc}
	newModel, _ = newModel.updateSubmit(msg)
	assert.NotNil(t, newModel)
    assert.Equal(t, viewMain, newModel.viewState)
}

func TestDashboardModel_UpdateConfirmation(t *testing.T) {
	mModel := NewDashboardModel("http://localhost:8080")
	mModel.viewState = viewConfirmation

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")}
	newModel, _ := mModel.updateConfirmation(msg)
	assert.NotNil(t, newModel)
    assert.Equal(t, viewMain, newModel.viewState)

	mModel.viewState = viewConfirmation
	msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")}
	newModel, cmd := mModel.updateConfirmation(msg)
	assert.NotNil(t, newModel)
	assert.Nil(t, cmd)
    assert.Equal(t, viewMain, newModel.viewState)

    mModel.viewState = viewConfirmation
	msg = tea.KeyMsg{Type: tea.KeyEsc}
	newModel, cmd = mModel.updateConfirmation(msg)
	assert.NotNil(t, newModel)
	assert.Nil(t, cmd)
    assert.Equal(t, viewMain, newModel.viewState)
}

func TestDashboardModel_View_AllStates(t *testing.T) {
    mModel := NewDashboardModel("http://localhost:8080")

    views := []viewState{
        viewMain,
        viewDetails,
        viewLogs,
        viewConfirmation,
        viewSubmit,
        viewAnalytics,
        viewTree,
        viewBlockers,
        viewDependents,
        viewTimeoutInput,
        viewDepsInput,
        viewEnvInput,
        viewTagsInput,
        viewAgentInput,
        viewRenameInput,
        viewExplain,
        viewCompare,
        viewSearchLogsInput,
        viewSearchLogsResult,
        viewTags,
    }

    for _, v := range views {
        mModel.viewState = v
        out := mModel.View()
        assert.NotEmpty(t, out)
    }
}

func TestDashboardModel_ArchiveKeys(t *testing.T) {
	columns := []table.Column{{Title: "ID", Width: 10}}
	tModel := table.New(table.WithColumns(columns))
	rows := []table.Row{{"JOB-1"}, {"JOB-2"}}
	tModel.SetRows(rows)
	tModel.SetCursor(0)

	model := DashboardModel{
		host:      "http://localhost",
		table:     tModel,
		viewState: viewMain,
		jobs: []orchestrator.JobInfo{
			{ID: "JOB-1", StartTime: time.Now()},
			{ID: "JOB-2", StartTime: time.Now()},
		},
	}

	t.Run("Archive Single Key (w)", func(t *testing.T) {
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("w")}
		updatedModel, cmd := model.Update(msg)
		assert.Nil(t, cmd)
		m, ok := updatedModel.(DashboardModel)
		assert.True(t, ok)
		assert.Equal(t, viewConfirmation, m.viewState)
		assert.Equal(t, "archive", m.pendingAction)
		assert.Equal(t, "JOB-1", m.pendingJobId)
	})

	t.Run("Archive Multiple Key (w)", func(t *testing.T) {
		m := model
		m.selectedJobs = map[string]bool{"JOB-1": true, "JOB-2": true}
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("w")}
		updatedModel, cmd := m.Update(msg)
		assert.Nil(t, cmd)
		m, ok := updatedModel.(DashboardModel)
		assert.True(t, ok)
		assert.Equal(t, viewConfirmation, m.viewState)
		assert.Equal(t, "archive multiple", m.pendingAction)
		assert.Equal(t, "MULTIPLE_w", m.pendingJobId)
	})
}
