package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func TestGitCleanupModel_Selection(t *testing.T) {
	branches := []BranchItem{
		{Name: "feature-1", Status: StatusActive},
		{Name: "feature-2", Status: StatusMerged},
	}

	m := NewGitCleanupModel(branches)
	m, _ = updateModel(m, tea.WindowSizeMsg{Width: 80, Height: 20})

	// Initial State
	assert.Empty(t, m.GetSelectedBranches())

	// Select first item (feature-1)
	// Press space
	m, _ = updateModel(m, tea.KeyMsg{Type: tea.KeySpace})
	assert.Contains(t, m.GetSelectedBranches(), "feature-1")
	assert.NotContains(t, m.GetSelectedBranches(), "feature-2")

	// Select second item
	// Move down
	m, _ = updateModel(m, tea.KeyMsg{Type: tea.KeyDown})
	// Press space
	m, _ = updateModel(m, tea.KeyMsg{Type: tea.KeySpace})
	assert.Contains(t, m.GetSelectedBranches(), "feature-1")
	assert.Contains(t, m.GetSelectedBranches(), "feature-2")

	// Deselect first item
	// Move up
	m, _ = updateModel(m, tea.KeyMsg{Type: tea.KeyUp})
	// Press space
	m, _ = updateModel(m, tea.KeyMsg{Type: tea.KeySpace})
	assert.NotContains(t, m.GetSelectedBranches(), "feature-1")
	assert.Contains(t, m.GetSelectedBranches(), "feature-2")
}

func TestGitCleanupModel_SelectAllMerged(t *testing.T) {
	branches := []BranchItem{
		{Name: "active-1", Status: StatusActive},
		{Name: "merged-1", Status: StatusMerged},
		{Name: "stale-1", Status: StatusStale},
	}

	m := NewGitCleanupModel(branches)
	m, _ = updateModel(m, tea.WindowSizeMsg{Width: 80, Height: 20})

	// Press 'a'
	m, _ = updateModel(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})

	selected := m.GetSelectedBranches()
	assert.Contains(t, selected, "merged-1")
	assert.Contains(t, selected, "stale-1")
	assert.NotContains(t, selected, "active-1")
}

func TestGitCleanupModel_Confirmation(t *testing.T) {
	branches := []BranchItem{
		{Name: "merged-1", Status: StatusMerged},
	}

	m := NewGitCleanupModel(branches)
	m, _ = updateModel(m, tea.WindowSizeMsg{Width: 80, Height: 20})

	// Select item
	m, _ = updateModel(m, tea.KeyMsg{Type: tea.KeySpace})

	// Press Enter
	m, _ = updateModel(m, tea.KeyMsg{Type: tea.KeyEnter})
	assert.True(t, m.confirming)
	assert.False(t, m.confirmed)
	assert.False(t, m.quitting)

	// Press 'n' (Cancel)
	m, _ = updateModel(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	assert.False(t, m.confirming)
	assert.False(t, m.quitting)

	// Press Enter again
	m, _ = updateModel(m, tea.KeyMsg{Type: tea.KeyEnter})
	assert.True(t, m.confirming)

	// Press 'y' (Confirm)
	m, cmd := updateModel(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	assert.True(t, m.confirmed)
	assert.True(t, m.quitting)
	assert.Equal(t, tea.Quit(), cmd())
}

// Helper to update model with type assertion
func updateModel(m GitCleanupModel, msg tea.Msg) (GitCleanupModel, tea.Cmd) {
	newM, cmd := m.Update(msg)
	return newM.(GitCleanupModel), cmd
}
