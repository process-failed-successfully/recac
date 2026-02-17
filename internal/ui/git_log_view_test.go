package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCommitItem_Methods(t *testing.T) {
	item := CommitItem{
		Hash:    "abc1234",
		Author:  "Bob",
		Date:    "yesterday",
		Message: "Fix bug",
	}

	assert.Contains(t, item.Title(), "abc1234")
	assert.Contains(t, item.Title(), "Fix bug")

	assert.Contains(t, item.Description(), "Bob")
	assert.Contains(t, item.Description(), "yesterday")

	assert.Contains(t, item.FilterValue(), "Fix bug")
	assert.Contains(t, item.FilterValue(), "abc1234")
	assert.Contains(t, item.FilterValue(), "Bob")
}

func TestGitLogModel_View(t *testing.T) {
	commits := []CommitItem{
		{Hash: "123", Message: "Initial commit"},
	}

	m := NewGitLogModel(commits, nil, nil, nil)

	// Set Size
	m.list.SetSize(80, 20)

	// Normal View
	view := m.View()
	assert.NotEmpty(t, view)
	assert.Contains(t, view, "Git Log")
	assert.Contains(t, view, "Initial commit")

	// Details View
	m.viewingDetails = true
	m.viewport.Width = 80
	m.viewport.Height = 20
	m.viewport.SetContent("Diff content")

	viewDetails := m.View()
	assert.Contains(t, viewDetails, "Commit Details")
	assert.Contains(t, viewDetails, "Diff content")
}

func TestGitLogModel_StatusView(t *testing.T) {
	m := NewGitLogModel([]CommitItem{}, nil, nil, nil)
	assert.Empty(t, m.statusView())

	m.statusMessage = "Loading..."
	assert.Contains(t, m.statusView(), "Loading...")
}

func TestGitLogModel_Init(t *testing.T) {
	m := NewGitLogModel([]CommitItem{}, nil, nil, nil)
	assert.Nil(t, m.Init())
}
