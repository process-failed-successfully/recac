package main

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestTodoCmd(t *testing.T) {
	// Setup temporary directory
	tmpDir, err := os.MkdirTemp("", "recac-todo-test")
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

	// Test 1: Add a task
	// We need to execute the command. Since we can't easily execute the main binary,
	// we'll call the RunE functions directly or use the command structure.
	// Since the commands rely on global flags or just args, we can call them.

	// Helper to capture output
	execute := func(cmdArgs []string) (string, error) {
		rootCmd.SetArgs(cmdArgs)
		buf := new(bytes.Buffer)
		rootCmd.SetOut(buf)
		rootCmd.SetErr(buf)
		err := rootCmd.Execute()
		return buf.String(), err
	}

	// 1. Add Task
	t.Run("Add Task", func(t *testing.T) {
		_, err := execute([]string{"todo", "add", "Buy milk"})
		if err != nil {
			t.Fatalf("Failed to add task: %v", err)
		}

		content, err := os.ReadFile("TODO.md")
		if err != nil {
			t.Fatalf("TODO.md not created")
		}
		if !strings.Contains(string(content), "- [ ] Buy milk") {
			t.Errorf("Task not found in file: %s", string(content))
		}
	})

	// 2. List Tasks
	t.Run("List Tasks", func(t *testing.T) {
		out, err := execute([]string{"todo", "list"})
		if err != nil {
			t.Fatalf("Failed to list tasks: %v", err)
		}
		if !strings.Contains(out, "1. [ ] Buy milk") {
			t.Errorf("List output incorrect: %s", out)
		}
	})

	// 3. Mark Done
	t.Run("Mark Done", func(t *testing.T) {
		_, err := execute([]string{"todo", "done", "1"})
		if err != nil {
			t.Fatalf("Failed to mark done: %v", err)
		}

		content, err := os.ReadFile("TODO.md")
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(content), "- [x] Buy milk") {
			t.Errorf("Task not marked done in file: %s", string(content))
		}

		out, err := execute([]string{"todo", "list"})
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out, "1. [x] Buy milk") {
			t.Errorf("List output incorrect after done: %s", out)
		}
	})

	// 4. Mark Undone
	t.Run("Mark Undone", func(t *testing.T) {
		_, err := execute([]string{"todo", "undone", "1"})
		if err != nil {
			t.Fatalf("Failed to mark undone: %v", err)
		}

		content, err := os.ReadFile("TODO.md")
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(content), "- [ ] Buy milk") {
			t.Errorf("Task not marked undone in file: %s", string(content))
		}
	})

	// 5. Remove Task
	t.Run("Remove Task", func(t *testing.T) {
		// Add another task first to ensure index handling
		execute([]string{"todo", "add", "Task 2"})

		_, err := execute([]string{"todo", "rm", "1"})
		if err != nil {
			t.Fatalf("Failed to remove task: %v", err)
		}

		content, err := os.ReadFile("TODO.md")
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(content), "Buy milk") {
			t.Errorf("Task 1 should be gone")
		}
		if !strings.Contains(string(content), "- [ ] Task 2") {
			t.Errorf("Task 2 should remain")
		}
	})

	// 6. Test File Creation if not exists
	t.Run("Create File", func(t *testing.T) {
		os.Remove("TODO.md")
		_, err := execute([]string{"todo", "add", "New Start"})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat("TODO.md"); os.IsNotExist(err) {
			t.Error("TODO.md was not recreated")
		}
	})
}
