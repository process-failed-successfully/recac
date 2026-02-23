package main

import (
	"os"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func TestTodoUiModel_Update(t *testing.T) {
	// Setup
	tmpDir, err := os.MkdirTemp("", "todo-ui-test")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)
	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	// Create TODO.md
	content := "# TODO\n\n- [ ] Task 1\n- [x] Task 2\n"
	err = os.WriteFile("TODO.md", []byte(content), 0644)
	assert.NoError(t, err)

	// Load items
	items, err := getTodoItems()
	assert.NoError(t, err)
	assert.Len(t, items, 2)

	m := newTodoUiModel(items)

	// Initialize list size to avoid panic or issues during Update if view is called
	m.list.SetSize(100, 20)

	// 1. Initial State
	assert.Equal(t, "Task 1", m.list.Items()[0].(todoItem).title)
	assert.False(t, m.list.Items()[0].(todoItem).done)
	assert.True(t, m.list.Items()[1].(todoItem).done)

	// 2. Test Toggle (Task 1)
	// Select first item (index 0)
	m.list.Select(0)

	// Send Toggle Key (Enter)
	msg := tea.KeyMsg{Type: tea.KeyEnter}

	model, cmd := m.Update(msg)
	m = model.(todoUiModel)

	// Verify command returned (it should be Batch(statusMsg, reloadListCmd))
	assert.NotNil(t, cmd)

	// Verify file updated
	bytes, _ := os.ReadFile("TODO.md")
	assert.Contains(t, string(bytes), "- [x] Task 1")

	// Simulate reload
	newItems, _ := getTodoItems()
	reloadMsg := itemsLoadedMsg{items: newItems}
	model, _ = m.Update(reloadMsg)
	m = model.(todoUiModel)

	assert.True(t, m.list.Items()[0].(todoItem).done, "Task 1 should be done in model")

	// 3. Test Delete (Task 2)
	// Move to Task 2. Since list items might have been reset, we ensure selection is valid.
	// list.SetItems resets selection to 0 usually? No, it tries to keep it.
	// But let's select index 1 explicitly.
	m.list.Select(1)

	// Send Delete Key (d)
	msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}}
	model, _ = m.Update(msg)
	m = model.(todoUiModel)

	// Verify file
	bytes, _ = os.ReadFile("TODO.md")
	assert.NotContains(t, string(bytes), "Task 2")
	assert.Contains(t, string(bytes), "- [x] Task 1")
}

func TestGetTodoItems(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "todo-items-test")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)
	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	content := `# TODO

- [ ] Simple Task
- [x] Done Task
- [ ] [main.go:10] Context Task
`
	os.WriteFile("TODO.md", []byte(content), 0644)

	items, err := getTodoItems()
	assert.NoError(t, err)
	assert.Len(t, items, 3)

	item1 := items[0].(todoItem)
	assert.Equal(t, "Simple Task", item1.title)
	assert.Equal(t, "", item1.desc)
	assert.False(t, item1.done)

	item3 := items[2].(todoItem)
	assert.Equal(t, "Context Task", item3.title)
	assert.Equal(t, "File: main.go:10", item3.desc)
	assert.Equal(t, "main.go", item3.filePath)
	assert.Equal(t, 10, item3.lineNum)
}
