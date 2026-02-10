package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"recac/internal/orchestrator"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTaskCmd(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "recac_task_test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	testWorkFile := filepath.Join(tmpDir, "work_items.json")

	t.Run("Add Task", func(t *testing.T) {
		output, err := executeCommand(rootCmd, "task", "add",
			"--file", testWorkFile,
			"--summary", "Test Task",
			"--description", "Do something",
			"--repo-url", "http://github.com/test/repo",
			"--id", "TEST-1",
		)
		require.NoError(t, err)
		assert.Contains(t, output, "Added task: TEST-1")

		// Verify file content
		data, err := os.ReadFile(testWorkFile)
		require.NoError(t, err)

		var items []orchestrator.WorkItem
		err = json.Unmarshal(data, &items)
		require.NoError(t, err)

		assert.Len(t, items, 1)
		assert.Equal(t, "TEST-1", items[0].ID)
		assert.Equal(t, "Test Task", items[0].Summary)
		assert.Equal(t, "http://github.com/test/repo", items[0].RepoURL)
	})

	t.Run("List Tasks", func(t *testing.T) {
		output, err := executeCommand(rootCmd, "task", "list", "--file", testWorkFile)
		require.NoError(t, err)
		assert.Contains(t, output, "TEST-1")
		assert.Contains(t, output, "Test Task")
		assert.Contains(t, output, "http://github.com/test/repo")
	})

	t.Run("Add Another Task Auto ID", func(t *testing.T) {
		output, err := executeCommand(rootCmd, "task", "add",
			"--file", testWorkFile,
			"--summary", "Task 2",
		)
		require.NoError(t, err)
		assert.Contains(t, output, "Added task: TASK-")

		// Verify file content
		data, err := os.ReadFile(testWorkFile)
		require.NoError(t, err)

		var items []orchestrator.WorkItem
		err = json.Unmarshal(data, &items)
		require.NoError(t, err)

		assert.Len(t, items, 2)
	})

	t.Run("List Empty", func(t *testing.T) {
		emptyFile := filepath.Join(tmpDir, "empty.json")
		output, err := executeCommand(rootCmd, "task", "list", "--file", emptyFile)
		require.NoError(t, err)
		assert.Contains(t, output, "No tasks found")
	})
}
