package main

import (
	"testing"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func TestInitialReviewModel(t *testing.T) {
	issues := []ReviewIssue{
		{
			Title:       "Test Issue",
			Description: "Test Desc",
			File:        "test.go",
			Line:        10,
			Severity:    "High",
		},
	}

	m := initialReviewModel(issues)

	assert.Equal(t, 1, len(m.issues))
	assert.Equal(t, "Test Issue", m.issues[0].Title)
	assert.NotNil(t, m.list)
}

func TestReviewModel_Update_Quit(t *testing.T) {
	m := initialReviewModel([]ReviewIssue{})
	msg := tea.KeyMsg{Type: tea.KeyCtrlC}

	newM, cmd := m.Update(msg)

	assert.NotNil(t, cmd)
	// Check if the command returns tea.QuitMsg
	msgCmd := cmd()
	assert.IsType(t, tea.QuitMsg{}, msgCmd)
	assert.Equal(t, m.ready, newM.(ReviewModel).ready)
}

func TestReviewModel_Update_WindowSize(t *testing.T) {
	m := initialReviewModel([]ReviewIssue{})
	msg := tea.WindowSizeMsg{Width: 100, Height: 50}

	newM, _ := m.Update(msg)
	updatedM := newM.(ReviewModel)

	assert.True(t, updatedM.ready)
	assert.Equal(t, 100, updatedM.width)
	assert.Equal(t, 50, updatedM.height)
}

func TestReviewModel_Update_Enter(t *testing.T) {
	issues := []ReviewIssue{
		{
			Title:       "Issue 1",
			Description: "Desc 1",
			File:        "file1.go",
		},
		{
			Title:       "Issue 2",
			Description: "Desc 2",
			File:        "file2.go",
		},
	}
	m := initialReviewModel(issues)

	// Simulate window size to init viewport
	newM, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 50})
	m = newM.(ReviewModel)

	// Simulate selecting an item (default is 0)
	msg := tea.KeyMsg{Type: tea.KeyEnter}
	newM2, _ := m.Update(msg)
	updatedM := newM2.(ReviewModel)

	assert.NotNil(t, updatedM.selectedIssue)
	assert.Equal(t, "Issue 1", updatedM.selectedIssue.Title)
}

func TestReviewModel_Update_FilteringIgnore(t *testing.T) {
	m := initialReviewModel([]ReviewIssue{
		{Title: "Test Issue"},
	})

	// Set state to filtering
	m.list.SetFilterState(list.Filtering)

	// Send Enter key, which should normally select the issue
	// But since we are filtering, it should be ignored by our switch
	msg := tea.KeyMsg{Type: tea.KeyEnter}
	newM, _ := m.Update(msg)

	// Assert no issue was selected
	assert.Nil(t, newM.(ReviewModel).selectedIssue)
}
