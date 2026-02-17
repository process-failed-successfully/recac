package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBranchItem_Methods(t *testing.T) {
	item := BranchItem{
		Name:       "feature/test",
		Status:     StatusActive,
		LastCommit: "2h ago",
		Author:     "Alice",
		IsSelected: false,
	}

	// Title
	assert.Equal(t, "[ ] feature/test", item.Title())

	item.IsSelected = true
	assert.Equal(t, "[x] feature/test", item.Title())

	// Description
	assert.Contains(t, item.Description(), "active")
	assert.Contains(t, item.Description(), "Alice")
	assert.Contains(t, item.Description(), "2h ago")

	// FilterValue
	assert.Contains(t, item.FilterValue(), "feature/test")
	assert.Contains(t, item.FilterValue(), "active")
	assert.Contains(t, item.FilterValue(), "Alice")
}

func TestGitCleanupModel_View(t *testing.T) {
	branches := []BranchItem{
		{Name: "feature-1", Status: StatusActive},
	}

	m := NewGitCleanupModel(branches)

	// Set Size
	m.list.SetSize(80, 20)

	// Normal View
	view := m.View()
	assert.NotEmpty(t, view)
	assert.Contains(t, view, "Git Branch Cleanup")
	assert.Contains(t, view, "feature-1")

	// Confirmation View
	m.confirming = true
	m.selectedItems = map[string]bool{"feature-1": true}

	viewConf := m.View()
	assert.Contains(t, viewConf, "Are you sure you want to delete 1 branches?")
	assert.Contains(t, viewConf, "(y/N)")
}

func TestBranchStatus_Constants(t *testing.T) {
	assert.Equal(t, BranchStatus("merged"), StatusMerged)
	assert.Equal(t, BranchStatus("unmerged"), StatusUnmerged)
	assert.Equal(t, BranchStatus("stale"), StatusStale)
	assert.Equal(t, BranchStatus("active"), StatusActive)
}

func TestGitCleanupModel_Update_Empty(t *testing.T) {
	m := NewGitCleanupModel([]BranchItem{})
	m.Init() // just to cover Init

	// Send unrelated message
	newM, cmd := m.Update(nil)
	assert.NotNil(t, newM)
	assert.Nil(t, cmd) // Wait, Update usually returns batch cmd which might be nil or empty batch
	// checking if it didn't crash
}
