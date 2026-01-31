package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func TestBoardModel_Navigation(t *testing.T) {
	todos := []TicketItem{{ID: "1", Summary: "T1"}}
	inProgress := []TicketItem{{ID: "2", Summary: "T2"}}
	dones := []TicketItem{{ID: "3", Summary: "T3"}}

	m := NewBoardModel(todos, inProgress, dones)
	m.Width = 100
	m.Height = 24

	// Initial: Focused on ToDo (0)
	if m.focused != 0 {
		t.Errorf("expected focused 0, got %d", m.focused)
	}

	// Right -> InProgress (1)
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m = newM.(BoardModel)
	if m.focused != 1 {
		t.Errorf("expected focused 1, got %d", m.focused)
	}

	// Right -> Done (2)
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m = newM.(BoardModel)
	if m.focused != 2 {
		t.Errorf("expected focused 2, got %d", m.focused)
	}

	// Right -> ToDo (0) (wrap)
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m = newM.(BoardModel)
	if m.focused != 0 {
		t.Errorf("expected focused 0, got %d", m.focused)
	}

	// Left -> Done (2) (wrap)
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	m = newM.(BoardModel)
	if m.focused != 2 {
		t.Errorf("expected focused 2, got %d", m.focused)
	}
}

func TestBoardModel_Selection(t *testing.T) {
	todos := []TicketItem{{ID: "1", Summary: "T1"}}
	m := NewBoardModel(todos, nil, nil)
	m.Width = 100
	m.Height = 24

	// Send Enter
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = newM.(BoardModel)

	if m.SelectedTicket == nil {
		t.Fatal("expected selected ticket, got nil")
	}
	if m.SelectedTicket.ID != "1" {
		t.Errorf("expected ID 1, got %s", m.SelectedTicket.ID)
	}
	if !m.Quitting {
		t.Error("expected Quitting to be true")
	}
}

func TestBoardModel_Init(t *testing.T) {
	m := NewBoardModel(nil, nil, nil)
	cmd := m.Init()
	assert.Nil(t, cmd)
}

func TestBoardModel_View(t *testing.T) {
	todos := []TicketItem{{ID: "1", Summary: "Task1"}}
	m := NewBoardModel(todos, nil, nil)

	// Simulate resize via Update to ensure all lists are resized
	msg := tea.WindowSizeMsg{Width: 100, Height: 30}
	newM, _ := m.Update(msg)
	m = newM.(BoardModel)

	view := m.View()
	assert.Contains(t, view, "To Do")
	assert.Contains(t, view, "Task1")
	assert.Contains(t, view, "In Progress")
	assert.Contains(t, view, "Done")

	// Test View with focus on InProgress
	m.focused = 1
	view = m.View()
	assert.Contains(t, view, "In Progress")

	// Test View with focus on Done
	m.focused = 2
	view = m.View()
	assert.Contains(t, view, "Done")

	// Test Quitting view
	m.Quitting = true
	assert.Equal(t, "", m.View())
}

func TestBoardModel_Update_WindowSize(t *testing.T) {
	m := NewBoardModel(nil, nil, nil)
	msg := tea.WindowSizeMsg{Width: 120, Height: 60}

	newM, _ := m.Update(msg)
	m = newM.(BoardModel)

	assert.Equal(t, 120, m.Width)
	assert.Equal(t, 60, m.Height)
}

func TestBoardModel_Update_SelectionAcrossLists(t *testing.T) {
	inProgress := []TicketItem{{ID: "2", Summary: "T2"}}
	dones := []TicketItem{{ID: "3", Summary: "T3"}}

	m := NewBoardModel(nil, inProgress, dones)

	// Switch to InProgress
	m.focused = 1
	// Select
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = newM.(BoardModel)
	assert.Equal(t, "2", m.SelectedTicket.ID)

	// Reset
	m = NewBoardModel(nil, inProgress, dones)
	// Switch to Done
	m.focused = 2
	// Select
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = newM.(BoardModel)
	assert.Equal(t, "3", m.SelectedTicket.ID)
}

func TestTicketItem_Methods(t *testing.T) {
	ti := TicketItem{ID: "1", Summary: "Sum", Desc: "Desc"}
	assert.Equal(t, "[1] Sum", ti.Title())
	assert.Equal(t, "Desc", ti.Description())
	assert.Equal(t, "Sum", ti.FilterValue())
}

func TestBoardModel_Quit(t *testing.T) {
	m := NewBoardModel(nil, nil, nil)
	msg := tea.KeyMsg{Type: tea.KeyCtrlC}
	newM, cmd := m.Update(msg)
	m = newM.(BoardModel)

	assert.True(t, m.Quitting)
	assert.Equal(t, tea.Quit(), cmd())
}
