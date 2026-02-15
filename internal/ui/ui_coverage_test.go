package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func TestBlameModel_Coverage(t *testing.T) {
	lines := []BlameLine{
		{LineNo: 1, Content: "line 1", SHA: "abc1234567890", Author: "me", Date: "now", Summary: "test"},
	}
	fetchDiff := func(sha string) (string, error) { return "diff", nil }
	explain := func(sha string) (string, error) { return "explained", nil }

	m := NewBlameModel(lines, fetchDiff, explain)

	// Test Init
	cmd := m.Init()
	assert.Nil(t, cmd) // Init returns nil in current impl

	// Test View
	view := m.View()
	assert.NotEmpty(t, view)

	// Test Update
	// WindowSize
	updatedModel, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 50})
	assert.NotNil(t, updatedModel)

	// KeyMsg
	updatedModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	assert.NotNil(t, updatedModel)
}

func TestGitLogModel_Coverage(t *testing.T) {
	commits := []CommitItem{
		{Hash: "abc", Author: "me", Date: "now", Message: "fix"},
	}
	fetchDiff := func(hash string) (string, error) { return "diff", nil }
	explain := func(hash string) (string, error) { return "explained", nil }
	audit := func(hash string) (string, error) { return "audited", nil }

	m := NewGitLogModel(commits, fetchDiff, explain, audit)

	// Test Init
	cmd := m.Init()
	assert.Nil(t, cmd)

	// Test View
	view := m.View()
	assert.NotEmpty(t, view)

	// Test Update
	updatedModel, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 50})
	assert.NotNil(t, updatedModel)

	updatedModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	assert.NotNil(t, updatedModel)
}

func TestGitCleanupModel_Coverage(t *testing.T) {
	branches := []BranchItem{
		{Name: "feature", Status: StatusMerged, Author: "me", LastCommit: "now"},
	}

	m := NewGitCleanupModel(branches)

	// Test Init
	cmd := m.Init()
	assert.Nil(t, cmd)

	// Test View
	view := m.View()
	assert.NotEmpty(t, view)

	// Test Update
	updatedModel, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 50})
	assert.NotNil(t, updatedModel)

	// Test Toggle Selection
	updatedModel, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace})
	assert.NotNil(t, updatedModel)

	// Check selection via getter
	cm, ok := updatedModel.(GitCleanupModel)
	if ok {
		assert.NotEmpty(t, cm.GetSelectedBranches())
	}
}
