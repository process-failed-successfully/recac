package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTicketItem_Methods(t *testing.T) {
	item := TicketItem{
		ID:      "PROJ-1",
		Summary: "Implement X",
		Desc:    "Details here",
		Status:  "To Do",
	}

	assert.Equal(t, "[PROJ-1] Implement X", item.Title())
	assert.Equal(t, "Details here", item.Description())
	assert.Equal(t, "Implement X", item.FilterValue())
}

func TestBoardModel_View(t *testing.T) {
	todos := []TicketItem{{ID: "1", Summary: "Task 1"}}
	inProgress := []TicketItem{{ID: "2", Summary: "Task 2"}}
	dones := []TicketItem{{ID: "3", Summary: "Task 3"}}

	m := NewBoardModel(todos, inProgress, dones)
	m.todo.SetSize(20, 10)
	m.inProgress.SetSize(20, 10)
	m.done.SetSize(20, 10)

	// Focus Todo
	m.focused = 0
	view0 := m.View()
	assert.Contains(t, view0, "Task 1")
	assert.Contains(t, view0, "Task 2")
	assert.Contains(t, view0, "Task 3")

	// Focus In Progress
	m.focused = 1
	view1 := m.View()
	assert.NotEmpty(t, view1)

	// Focus Done
	m.focused = 2
	view2 := m.View()
	assert.NotEmpty(t, view2)

	// Quitting
	m.Quitting = true
	assert.Empty(t, m.View())
}

func TestBoardModel_Init(t *testing.T) {
	m := NewBoardModel(nil, nil, nil)
	assert.Nil(t, m.Init())
}
