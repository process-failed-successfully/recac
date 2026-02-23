package main

import (
	"os"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func TestTodoUI_Navigation(t *testing.T) {
	// Setup
	tmpFile, err := os.CreateTemp("", "todo-ui-test-*.md")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())

	// Override todoFilename
	oldFilename := todoFilename
	todoFilename = tmpFile.Name()
	defer func() { todoFilename = oldFilename }()

	// Add some tasks
	tasks := []TodoTask{
		{Description: "Task 1", Done: false},
		{Description: "Task 2", Done: false},
		{Description: "Task 3", Done: false},
	}
	saveTasks(tasks)

	// Init model
	model := initialTodoModel()

	// Initial state
	assert.Equal(t, 0, model.cursor)

	// Move down (j)
	model, _ = updateModel(model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	assert.Equal(t, 1, model.cursor)

	// Move down (down arrow)
	model, _ = updateModel(model, tea.KeyMsg{Type: tea.KeyDown})
	assert.Equal(t, 2, model.cursor)

	// Move down (bound check)
	model, _ = updateModel(model, tea.KeyMsg{Type: tea.KeyDown})
	assert.Equal(t, 2, model.cursor)

	// Move up (k)
	model, _ = updateModel(model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	assert.Equal(t, 1, model.cursor)

	// Move up (up arrow)
	model, _ = updateModel(model, tea.KeyMsg{Type: tea.KeyUp})
	assert.Equal(t, 0, model.cursor)
}

func TestTodoUI_Toggle(t *testing.T) {
	// Setup
	tmpFile, err := os.CreateTemp("", "todo-ui-test-*.md")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())

	oldFilename := todoFilename
	todoFilename = tmpFile.Name()
	defer func() { todoFilename = oldFilename }()

	saveTasks([]TodoTask{{Description: "Task 1", Done: false}})

	model := initialTodoModel()

	// Toggle (space)
	model, _ = updateModel(model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(" ")})
	assert.True(t, model.tasks[0].Done)

	// Verify file
	tasks, _ := loadTasks()
	assert.True(t, tasks[0].Done)

	// Toggle back (enter)
	model, _ = updateModel(model, tea.KeyMsg{Type: tea.KeyEnter})
	assert.False(t, model.tasks[0].Done)
}

func TestTodoUI_Delete(t *testing.T) {
	// Setup
	tmpFile, err := os.CreateTemp("", "todo-ui-test-*.md")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())

	oldFilename := todoFilename
	todoFilename = tmpFile.Name()
	defer func() { todoFilename = oldFilename }()

	saveTasks([]TodoTask{
		{Description: "Task 1", Done: false},
		{Description: "Task 2", Done: false},
	})

	model := initialTodoModel()

	// Delete (d) at cursor 0
	model, _ = updateModel(model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})

	assert.Len(t, model.tasks, 1)
	assert.Equal(t, "Task 2", model.tasks[0].Description)
	assert.Equal(t, 0, model.cursor)

	// Verify file
	tasks, _ := loadTasks()
	assert.Len(t, tasks, 1)
	assert.Equal(t, "Task 2", tasks[0].Description)
}

func TestTodoUI_Add(t *testing.T) {
	// Setup
	tmpFile, err := os.CreateTemp("", "todo-ui-test-*.md")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())

	oldFilename := todoFilename
	todoFilename = tmpFile.Name()
	defer func() { todoFilename = oldFilename }()

	saveTasks([]TodoTask{})

	model := initialTodoModel()

	// Enter add mode (a)
	model, cmd := updateModel(model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	assert.True(t, model.adding)
	// Focus command should be returned (textinput.Blink)
	assert.NotNil(t, cmd)

	// Type "New Task"
	// We need to simulate typing. textinput uses tea.KeyMsg for runes.
	input := "New Task"
	for _, r := range input {
		model, _ = updateModel(model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}

	assert.Equal(t, "New Task", model.input.Value())

	// Press Enter
	model, _ = updateModel(model, tea.KeyMsg{Type: tea.KeyEnter})

	assert.False(t, model.adding)
	assert.Len(t, model.tasks, 1)
	assert.Equal(t, "New Task", model.tasks[0].Description)
	assert.Equal(t, "", model.input.Value()) // Should be reset

	// Verify file
	tasks, _ := loadTasks()
	assert.Len(t, tasks, 1)
	assert.Equal(t, "New Task", tasks[0].Description)
}

// Helper to cast model back to TodoModel
func updateModel(m TodoModel, msg tea.Msg) (TodoModel, tea.Cmd) {
	newModel, cmd := m.Update(msg)
	return newModel.(TodoModel), cmd
}
