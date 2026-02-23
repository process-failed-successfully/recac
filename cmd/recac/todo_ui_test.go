package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"recac/internal/utils"
)

func TestTodoUiCmdRegistration(t *testing.T) {
	// Verify that "ui" is a subcommand of "todo"
	found := false
	for _, cmd := range todoCmd.Commands() {
		if cmd.Use == "ui" {
			found = true
			break
		}
	}
	if !found {
		t.Error("ui subcommand not found in todoCmd")
	}
}

func TestTodoModel_Integration(t *testing.T) {
	// Setup temp file
	tmpDir, err := os.MkdirTemp("", "recac-todo-ui-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Override todoFilename
	originalFilename := todoFilename
	todoFilename = filepath.Join(tmpDir, "TODO.md")
	defer func() { todoFilename = originalFilename }()

	// Create initial TODO file
	initialContent := []string{
		"- [ ] Task 1",
		"- [x] Task 2",
		"- [ ] Task 3",
	}
	if err := utils.WriteLines(todoFilename, initialContent); err != nil {
		t.Fatal(err)
	}

	// Helper to create model
	createModel := func() todoModel {
		items, err := loadTodoItems()
		if err != nil {
			t.Fatal(err)
		}
		l := list.New(items, list.NewDefaultDelegate(), 40, 20)
		ti := textinput.New()
		return todoModel{list: l, input: ti}
	}

	// Test 1: Toggle Task 1 (index 1)
	t.Run("Toggle Task", func(t *testing.T) {
		m := createModel()
		// Select first item (Task 1) - default selection is 0
		// Send "space"
		m.Update(tea.KeyMsg{Type: tea.KeySpace})

		// Verify file change
		// Note: Update returns commands, which might be async. But toggleTaskStatus is synchronous file write.
		// However, m.Update returns tea.Cmd which reloads tasks. The reload is async.
		// But the file write happens synchronously inside Update -> toggleTaskStatus.

		content, err := utils.ReadLines(todoFilename)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(content[0], "- [x] Task 1") {
			t.Errorf("Task 1 should be toggled to [x], got: %s", content[0])
		}
	})

	// Test 2: Delete Task 2 (index 2)
	t.Run("Delete Task", func(t *testing.T) {
		// Reset file
		utils.WriteLines(todoFilename, initialContent)
		m := createModel()

		// Move down to second item
		m.list.Select(1)

		// Send "d"
		m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})

		content, err := utils.ReadLines(todoFilename)
		if err != nil {
			t.Fatal(err)
		}
		if len(content) != 2 {
			t.Errorf("Expected 2 lines, got %d", len(content))
		}
		if strings.Contains(strings.Join(content, "\n"), "Task 2") {
			t.Error("Task 2 should be deleted")
		}
	})

	// Test 3: Add Task
	t.Run("Add Task", func(t *testing.T) {
		// Reset file
		utils.WriteLines(todoFilename, initialContent)
		m := createModel()

		// Press 'n' to enter add mode
		newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
		m = newM.(todoModel)

		if !m.adding {
			t.Error("Expected adding mode to be true")
		}

		// Type "Task 4"
		// We need to simulate typing into textinput
		// We can just set the value directly for simplicity in test,
		// or simulate keys. Simulating keys is safer.
		for _, r := range "Task 4" {
			newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
			m = newM.(todoModel)
		}

		// Press Enter
		newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
		m = newM.(todoModel)

		if m.adding {
			t.Error("Expected adding mode to be false")
		}

		// Verify file
		content, err := utils.ReadLines(todoFilename)
		if err != nil {
			t.Fatal(err)
		}
		if len(content) != 4 {
			t.Errorf("Expected 4 lines, got %d", len(content))
		}
		lastLine := content[len(content)-1]
		if !strings.Contains(lastLine, "- [ ] Task 4") {
			t.Errorf("Expected last line to be '- [ ] Task 4', got: %s", lastLine)
		}
	})
}

// Mocking time.Sleep if needed? No, file ops are fast enough.
