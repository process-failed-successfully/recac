package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"recac/internal/orchestrator"
)

func TestTaskCommands(t *testing.T) {
	tempDir := t.TempDir()
	workFile := filepath.Join(tempDir, "tasks.json")

	// Setup viper config
	viper.Set("orchestrator.work_file", workFile)

	// 1. Add Task
	t.Run("Add Task", func(t *testing.T) {
		taskAddCmd.Flags().Set("summary", "Fix Bug")
		taskAddCmd.Flags().Set("description", "Desc")
		taskAddCmd.Flags().Set("repo-url", "http://repo")

		err := taskAddCmd.RunE(taskAddCmd, []string{})
		assert.NoError(t, err)

		// Verify file
		content, err := os.ReadFile(workFile)
		assert.NoError(t, err)
		var items []orchestrator.WorkItem
		err = json.Unmarshal(content, &items)
		assert.NoError(t, err)
		assert.Len(t, items, 1)
		assert.Equal(t, "Fix Bug", items[0].Summary)
		assert.Equal(t, "Desc", items[0].Description)
		assert.Equal(t, "http://repo", items[0].RepoURL)
	})

	// 2. Add Another Task (with Env)
	t.Run("Add Task With Env", func(t *testing.T) {
		taskAddCmd.Flags().Set("summary", "Feature X")
		taskAddCmd.Flags().Set("repo-url", "http://repo2")
		taskAddCmd.Flags().Set("env", "KEY=VALUE")

		err := taskAddCmd.RunE(taskAddCmd, []string{})
		assert.NoError(t, err)

		content, _ := os.ReadFile(workFile)
		var items []orchestrator.WorkItem
		json.Unmarshal(content, &items)
		assert.Len(t, items, 2)
		assert.Equal(t, "Feature X", items[1].Summary)
		assert.Equal(t, "VALUE", items[1].EnvVars["KEY"])
	})

	// 3. List Tasks
	t.Run("List Tasks", func(t *testing.T) {
		// We capture stdout if we want to verify output, but RunE returns nil error
		// which verifies logic didn't crash
		err := taskListCmd.RunE(taskListCmd, []string{})
		assert.NoError(t, err)
	})

	// 4. Clear Tasks
	t.Run("Clear Tasks", func(t *testing.T) {
		err := taskClearCmd.RunE(taskClearCmd, []string{})
		assert.NoError(t, err)

		content, _ := os.ReadFile(workFile)
		var items []orchestrator.WorkItem
		json.Unmarshal(content, &items)
		assert.Len(t, items, 0)
	})
}
