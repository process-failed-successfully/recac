package main

import (
	"os"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

func TestTodoUiModel(t *testing.T) {
	// Setup temporary directory
	tmpDir, err := os.MkdirTemp("", "recac-todo-ui-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Save current working directory and restore it after test
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(originalWd)

	// Change to temp dir
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	// Create TODO.md with a task
	// Use [file:line] format to test solving trigger
	err = os.WriteFile("TODO.md", []byte("- [ ] [main.go:10] Fix bug\n- [ ] Simple task\n"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	// Init model
	tasks, err := getTodoItems()
	if err != nil {
		t.Fatal(err)
	}
	items := make([]list.Item, len(tasks))
	for i, t := range tasks {
		items[i] = todoItem{t}
	}

	delegate := list.NewDefaultDelegate()
	l := list.New(items, delegate, 0, 0)
	m := todoUiModel{list: l}

	// Test 1: Space to toggle
	t.Run("Space Toggles Status", func(t *testing.T) {
		m.list.Select(0)
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}}
		newModel, cmd := m.Update(msg)
		_ = newModel

		// Verify file changed
		content, _ := os.ReadFile("TODO.md")
		if !strings.Contains(string(content), "- [x] [main.go:10] Fix bug") {
			t.Errorf("Task 1 not toggled: %s", string(content))
		}

		if cmd == nil {
			t.Error("Expected cmd to reload list")
		}
	})

	// Test 2: Enter on task with file context triggers solving
	t.Run("Enter Triggers Solving", func(t *testing.T) {
		// reload items to reflect changes (though toggle modifies file, model list is stale until cmd runs)
		// Manually update model list for test
		tasks, _ := getTodoItems()
		items := make([]list.Item, len(tasks))
		for i, t := range tasks {
			items[i] = todoItem{t}
		}
		m.list.SetItems(items)
		m.list.Select(0) // Has file context (index 0 corresponds to first task)

		msg := tea.KeyMsg{Type: tea.KeyEnter}
		newModel, cmd := m.Update(msg)

		mm := newModel.(todoUiModel)
		if !mm.solving {
			t.Error("Expected solving state to be true")
		}
		if cmd == nil {
			t.Error("Expected cmd to start solving")
		}
	})

	// Test 3: Enter on task WITHOUT file context
	t.Run("Enter No File Context", func(t *testing.T) {
		m.list.Select(1) // Simple task (index 1)

		msg := tea.KeyMsg{Type: tea.KeyEnter}
		newModel, cmd := m.Update(msg)

		mm := newModel.(todoUiModel)
		if mm.solving {
			t.Error("Should not start solving without file context")
		}
		// Expect a status message command (which is tea.Cmd)
		// bubbles/list NewStatusMessage returns a Cmd
		if cmd == nil {
			t.Error("Expected cmd to show status message")
		}
	})
}
