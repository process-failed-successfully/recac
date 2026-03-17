package tui

import (
	"fmt"
	"recac/internal/orchestrator"
	"testing"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func TestDashboardModel_Update_CompareKey_Success(t *testing.T) {
	// Setup
	columns := []table.Column{{Title: "ID", Width: 10}}
	tModel := table.New(table.WithColumns(columns))

	model := DashboardModel{
		host:         "http://localhost",
		table:        tModel,
		viewState:    viewMain,
		selectedJobs: map[string]bool{"JOB-1": true, "JOB-2": true},
	}

	// Act: Press "="
	updatedModel, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("=")})
	m := updatedModel.(DashboardModel)

	// Assert: Command should be returned (fetchCompareJobs)
	assert.NotNil(t, cmd)
	// View state shouldn't change yet (until msg received)
	assert.Equal(t, viewMain, m.viewState)
	assert.Nil(t, m.err)
}

func TestDashboardModel_Update_CompareKey_ErrorNotTwoJobs(t *testing.T) {
	// Setup
	columns := []table.Column{{Title: "ID", Width: 10}}
	tModel := table.New(table.WithColumns(columns))

	model := DashboardModel{
		host:         "http://localhost",
		table:        tModel,
		viewState:    viewMain,
		selectedJobs: map[string]bool{"JOB-1": true}, // Only 1 job selected
	}

	// Act: Press "="
	updatedModel, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("=")})
	m := updatedModel.(DashboardModel)

	// Assert: Command should be nil
	assert.Nil(t, cmd)
	// View state shouldn't change
	assert.Equal(t, viewMain, m.viewState)
	// Error should be set
	assert.NotNil(t, m.err)
	assert.Equal(t, "Exactly 2 jobs must be selected to compare", m.err.Error())
}

func TestDashboardModel_Update_CompareMsg_Success(t *testing.T) {
	// Setup
	model := DashboardModel{
		viewState: viewMain,
	}

	job1 := orchestrator.JobInfo{ID: "JOB-1", Summary: "Summary 1"}
	job2 := orchestrator.JobInfo{ID: "JOB-2", Summary: "Summary 2"}

	msg := compareMsg{
		Jobs: [2]orchestrator.JobInfo{job1, job2},
		Err:  nil,
	}

	// Act
	updatedModel, cmd := model.Update(msg)
	m := updatedModel.(DashboardModel)

	// Assert
	assert.Nil(t, cmd)
	assert.Equal(t, viewCompare, m.viewState)
	assert.Equal(t, job1, m.compareJobs[0])
	assert.Equal(t, job2, m.compareJobs[1])
	assert.Nil(t, m.err)
}

func TestDashboardModel_Update_CompareMsg_Error(t *testing.T) {
	// Setup
	model := DashboardModel{
		viewState: viewMain,
	}

	msg := compareMsg{
		Err: fmt.Errorf("fetch error"),
	}

	// Act
	updatedModel, cmd := model.Update(msg)
	m := updatedModel.(DashboardModel)

	// Assert
	assert.Nil(t, cmd)
	assert.Equal(t, viewMain, m.viewState) // View doesn't change on error
	assert.NotNil(t, m.err)
	assert.Equal(t, "fetch error", m.err.Error())
}
