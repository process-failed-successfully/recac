package main

import (
	"os"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func TestTodoUi_Update(t *testing.T) {
	// Setup temp dir
	tmpDir, err := os.MkdirTemp("", "recac-todo-ui-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(originalWd)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	// Create initial TODO.md
	err = os.WriteFile("TODO.md", []byte("# TODO\n\n- [ ] Task 1\n"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	model, err := NewTodoModel()
	if err != nil {
		t.Fatal(err)
	}

	// 1. Test Toggle Status (Task 1 is initially unchecked)
	// Task 1 should be selected by default (index 0)

	// Send "enter" to toggle
	updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	tm := updatedModel.(TodoModel)

	items := tm.list.Items()
	assert.Equal(t, 1, len(items))
	assert.True(t, items[0].(Task).Done)

	// Verify file content updated
	content, _ := os.ReadFile("TODO.md")
	assert.Contains(t, string(content), "- [x] Task 1")

	// 2. Test Add Task
	// Send "a" to start adding
	updatedModel, _ = tm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	tm = updatedModel.(TodoModel)
	assert.True(t, tm.adding)

	// Set input value directly to simulate typing
    tm.input.SetValue("New Task")

    // Send enter to confirm add
    updatedModel, _ = tm.Update(tea.KeyMsg{Type: tea.KeyEnter})
    tm = updatedModel.(TodoModel)

    assert.False(t, tm.adding)
    items = tm.list.Items()
    assert.Equal(t, 2, len(items))
    assert.Equal(t, "New Task", items[1].(Task).Desc)

    content, _ = os.ReadFile("TODO.md")
    assert.Contains(t, string(content), "- [ ] New Task")

    // 3. Test Delete Task
    // Select the second task (index 1)
    // List model handles "j" to move down.
    updatedModel, _ = tm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
    tm = updatedModel.(TodoModel)
    assert.Equal(t, 1, tm.list.Index())

    // Send "d" to delete
    updatedModel, _ = tm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
    tm = updatedModel.(TodoModel)

    items = tm.list.Items()
    assert.Equal(t, 1, len(items))
    assert.Equal(t, "Task 1", items[0].(Task).Desc) // Task 1 remains

    content, _ = os.ReadFile("TODO.md")
    assert.NotContains(t, string(content), "New Task")
}
