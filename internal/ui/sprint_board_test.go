package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestSprintBoardModel_AddTask(t *testing.T) {
	board := NewSprintBoardModel()

	board.AddTask("task-1", "Test Task 1")
	board.AddTask("task-2", "Test Task 2")

	if len(board.tasks) != 2 {
		t.Errorf("Expected 2 tasks, got %d", len(board.tasks))
	}

	if board.tasks[0].ID != "task-1" || board.tasks[0].Name != "Test Task 1" {
		t.Errorf("First task not added correctly")
	}

	if board.tasks[0].Status != TaskPending {
		t.Errorf("Expected task status to be Pending, got %v", board.tasks[0].Status)
	}
}

func TestSprintBoardModel_UpdateTaskStatus(t *testing.T) {
	board := NewSprintBoardModel()
	board.width = 100
	board.height = 30
	board.ready = true

	board.AddTask("task-1", "Test Task")

	// Update task status to In Progress
	updated, _ := board.Update(SprintBoardMsg{TaskID: "task-1", Status: TaskInProgress})
	board = updated.(SprintBoardModel)

	if board.tasks[0].Status != TaskInProgress {
		t.Errorf("Expected task status to be InProgress, got %v", board.tasks[0].Status)
	}

	// Update task status to Done
	updated, _ = board.Update(SprintBoardMsg{TaskID: "task-1", Status: TaskDone})
	board = updated.(SprintBoardModel)

	if board.tasks[0].Status != TaskDone {
		t.Errorf("Expected task status to be Done, got %v", board.tasks[0].Status)
	}
}

func TestSprintBoardModel_ViewColumns(t *testing.T) {
	board := NewSprintBoardModel()
	board.width = 120
	board.height = 30
	board.ready = true

	board.AddTask("task-1", "Pending Task")
	board.AddTask("task-2", "In Progress Task")
	board.AddTask("task-3", "Done Task")

	// Set task statuses
	updated, _ := board.Update(SprintBoardMsg{TaskID: "task-2", Status: TaskInProgress})
	board = updated.(SprintBoardModel)

	updated, _ = board.Update(SprintBoardMsg{TaskID: "task-3", Status: TaskDone})
	board = updated.(SprintBoardModel)

	view := board.View()

	// Verify columns are present
	if !strings.Contains(view, "Pending") {
		t.Error("Expected 'Pending' column in view")
	}

	if !strings.Contains(view, "In Progress") {
		t.Error("Expected 'In Progress' column in view")
	}

	if !strings.Contains(view, "Done") {
		t.Error("Expected 'Done' column in view")
	}

	// Verify tasks appear in correct columns
	if !strings.Contains(view, "Pending Task") {
		t.Error("Expected 'Pending Task' in view")
	}

	if !strings.Contains(view, "In Progress Task") {
		t.Error("Expected 'In Progress Task' in view")
	}

	if !strings.Contains(view, "Done Task") {
		t.Error("Expected 'Done Task' in view")
	}
}

func TestSprintBoardModel_TaskMovement(t *testing.T) {
	board := NewSprintBoardModel()
	board.width = 120
	board.height = 30
	board.ready = true

	board.AddTask("task-1", "Moving Task")

	// Start as Pending
	view1 := board.View()
	if !strings.Contains(view1, "Moving Task") {
		t.Error("Task should appear in Pending column initially")
	}

	// Move to In Progress
	updated, _ := board.Update(SprintBoardMsg{TaskID: "task-1", Status: TaskInProgress})
	board = updated.(SprintBoardModel)

	if board.tasks[0].Status != TaskInProgress {
		t.Error("Task should be in InProgress status")
	}

	// Move to Done
	updated, _ = board.Update(SprintBoardMsg{TaskID: "task-1", Status: TaskDone})
	board = updated.(SprintBoardModel)

	if board.tasks[0].Status != TaskDone {
		t.Error("Task should be in Done status")
	}
}

func TestSprintBoardModel_Quit(t *testing.T) {
	board := NewSprintBoardModel()

	// Test quit command
	_, cmd := board.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})

	if cmd == nil {
		t.Error("Expected quit command on 'q' keypress")
	}
}
