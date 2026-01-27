package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// --- TicketItem Tests ---

func TestTicketItem_Methods(t *testing.T) {
	item := TicketItem{ID: "PROJ-1", Summary: "Fix bug", Desc: "Details", Status: "To Do"}

	assert.Equal(t, "[PROJ-1] Fix bug", item.Title())
	assert.Equal(t, "Details", item.Description())
	assert.Equal(t, "Fix bug", item.FilterValue())
}

// --- BoardModel Tests ---

func TestBoardModel_View(t *testing.T) {
	todos := []TicketItem{{ID: "1", Summary: "Task 1"}}
	m := NewBoardModel(todos, nil, nil)
	m.Width = 100
	m.Height = 20
	// Init sizes (usually done in Update on WindowSizeMsg)
	m.todo.SetSize(30, 10)
	m.inProgress.SetSize(30, 10)
	m.done.SetSize(30, 10)

	// Case 1: Focus Todo
	m.focused = 0
	view := m.View()
	assert.Contains(t, view, "Task 1")
	assert.Contains(t, view, "To Do")

	// Case 2: Focus InProgress
	m.focused = 1
	view = m.View()
	assert.NotEmpty(t, view)

	// Case 3: Focus Done
	m.focused = 2
	view = m.View()
	assert.NotEmpty(t, view)

	// Case 4: Quitting
	m.Quitting = true
	assert.Empty(t, m.View())
}

// --- BranchItem Tests ---

func TestBranchItem_Methods(t *testing.T) {
	item := BranchItem{Name: "feature", Status: StatusActive, Author: "Me", LastCommit: "Now", IsSelected: false}

	assert.Equal(t, "[ ] feature", item.Title())
	assert.Contains(t, item.Description(), "active")
	assert.Contains(t, item.Description(), "Me")
	assert.Contains(t, item.FilterValue(), "feature")

	item.IsSelected = true
	assert.Equal(t, "[x] feature", item.Title())
}

// --- GitCleanupModel Tests ---

func TestGitCleanupModel_View(t *testing.T) {
	items := []BranchItem{{Name: "b1"}}
	m := NewGitCleanupModel(items)
	m.list.SetSize(20, 10)

	// Normal view
	view := m.View()
	assert.Contains(t, view, "b1")

	// Confirmation view
	m.selectedItems["b1"] = true
	m.confirming = true
	view = m.View()
	assert.Contains(t, view, "Are you sure")
	assert.Contains(t, view, "delete 1 branches")
}

func TestGitCleanupModel_GetSelectedBranches(t *testing.T) {
	items := []BranchItem{{Name: "b1"}, {Name: "b2"}}
	m := NewGitCleanupModel(items)
	m.selectedItems["b1"] = true
	m.selectedItems["b2"] = false

	selected := m.GetSelectedBranches()
	assert.Len(t, selected, 1)
	assert.Equal(t, "b1", selected[0])
}

// --- CommitItem Tests ---

func TestCommitItem_Methods(t *testing.T) {
	item := CommitItem{Hash: "abc", Message: "fix", Author: "Dev", Date: "Today"}

	assert.Equal(t, "abc - fix", item.Title())
	assert.Equal(t, "Dev | Today", item.Description())
	assert.Contains(t, item.FilterValue(), "fix")
}

// --- GitLogModel Tests ---

func TestGitLogModel_View(t *testing.T) {
	items := []CommitItem{{Hash: "123", Message: "Init"}}
	m := NewGitLogModel(items, nil, nil, nil)
	m.list.SetSize(20, 10)
	m.viewport.Width = 20
	m.viewport.Height = 10

	// List view
	view := m.View()
	assert.Contains(t, view, "Init")

	// Status message
	m.statusMessage = "Loading..."
	view = m.View()
	assert.Contains(t, view, "Loading...")

	// Details view
	m.viewingDetails = true
	m.viewport.SetContent("Diff content")
	view = m.View()
	assert.Contains(t, view, "Diff content")
	assert.Contains(t, view, "Commit Details")
}

// --- FileItem Tests ---

func TestFileItem_Methods(t *testing.T) {
	// Mock file info
	f := FileItem{Name: "file.txt", IsDir: false, DescStr: "1KB"}
	assert.Equal(t, "📄 file.txt", f.Title())
	assert.Equal(t, "1KB", f.Description())
	assert.Equal(t, "file.txt", f.FilterValue())

	d := FileItem{Name: "dir", IsDir: true}
	assert.Equal(t, "📁 dir", d.Title())
}
