package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestTodoModel_Toggle(t *testing.T) {
	// Setup
	tmpDir := t.TempDir()
	file := filepath.Join(tmpDir, "TODO.md")
	content := "# TODO\n\n- [ ] Task 1\n- [x] Task 2\n- [ ] Task 3\n"
	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// Initialize Model
	m, err := NewTodoModel(file)
	if err != nil {
		t.Fatal(err)
	}

	// Verify Initial State
	if len(m.items) != 3 {
		t.Errorf("Expected 3 items, got %d", len(m.items))
	}
	if m.items[0].Done {
		t.Error("Task 1 should be undone")
	}
	if !m.items[1].Done {
		t.Error("Task 2 should be done")
	}

	// Simulate "Enter" on first item (Task 1) -> Should become Done
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = newM.(TodoModel)

	// Verify Memory State
	if !m.items[0].Done {
		t.Error("Task 1 should now be done")
	}

	// Verify File State
	fileContent, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(fileContent), "- [x] Task 1") {
		t.Error("File should be updated with Task 1 done")
	}

	// Move down to Task 2
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = newM.(TodoModel)

	// Now select (Task 2) -> Should become Undone
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = newM.(TodoModel)

	if m.items[1].Done {
		t.Error("Task 2 should now be undone")
	}

	fileContent, _ = os.ReadFile(file)
	if !strings.Contains(string(fileContent), "- [ ] Task 2") {
		t.Error("File should be updated with Task 2 undone")
	}
}

func TestTodoModel_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	file := filepath.Join(tmpDir, "EMPTY_TODO.md")

	// Don't create file

	m, err := NewTodoModel(file)
	if err != nil {
		t.Fatal(err)
	}

	if len(m.items) != 0 {
		t.Errorf("Expected 0 items, got %d", len(m.items))
	}
}
