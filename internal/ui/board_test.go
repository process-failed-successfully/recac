package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
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
