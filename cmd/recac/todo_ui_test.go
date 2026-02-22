package main

import (
	"os"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func TestTodoUIModel_Init(t *testing.T) {
	// Setup TODO.md
	tmpDir, err := os.MkdirTemp("", "recac-todo-ui-test")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	err = os.WriteFile("TODO.md", []byte("# TODO\n\n- [ ] Task 1\n- [x] [main.go:10] Task 2\n"), 0644)
	assert.NoError(t, err)

	m := newTodoUIModel()
	cmd := m.Init()
	assert.NotNil(t, cmd)

	// Run the command manually
	msg := cmd()
	tasksMsg, ok := msg.(taskListMsg)
	assert.True(t, ok)
	assert.Len(t, tasksMsg, 2)
	assert.Equal(t, "Task 1", tasksMsg[0].title)
	assert.Equal(t, "Task 2", tasksMsg[1].title)
	assert.Equal(t, "main.go", tasksMsg[1].file)
	assert.Equal(t, 10, tasksMsg[1].line)
}

func TestTodoUIModel_Update_Viewport(t *testing.T) {
	// Setup
	tmpDir, err := os.MkdirTemp("", "recac-todo-ui-vp-test")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	err = os.WriteFile("main.go", []byte("line 1\nline 2\nline 3\n..."), 0644)
	assert.NoError(t, err)

	items := []*todoItem{
		{title: "Task 1", index: 1},
		{title: "Task 2", file: "main.go", line: 2, index: 2},
	}

	m := newTodoUIModel()
	// Simulate window size first to init viewport
	newM, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
	m = newM.(todoUIModel)

	// Load items
	newM, _ = m.Update(taskListMsg(items))
	m = newM.(todoUIModel)

	// Verify first item selected
	assert.Equal(t, 0, m.list.Index())
	// Check viewport content for item 0
	assert.Contains(t, m.viewport.View(), "No file context")

	// Move down to item 1
	// Create a KeyDown msg
	keyDown := tea.KeyMsg{Type: tea.KeyDown, Runes: []rune{}, Alt: false}
	newM, _ = m.Update(keyDown)
	m = newM.(todoUIModel)

	// Verify selection changed
	assert.Equal(t, 1, m.list.Index())

	// Check viewport content for item 1
	// It should contain "line 2" and highlighting ">"
	viewContent := m.viewport.View()
	assert.Contains(t, viewContent, "line 2")
	assert.Contains(t, viewContent, "> 2: line 2")
}
