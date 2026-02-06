package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"recac/internal/orchestrator"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestTaskCommands(t *testing.T) {
	// Setup temporary work file
	tmpDir := t.TempDir()
	workFile := filepath.Join(tmpDir, "test_work_items.json")
	viper.Set("orchestrator.work_file", workFile)

	// Test Add
	t.Run("Add Task", func(t *testing.T) {
		cmd := &cobra.Command{}
		cmd.Flags().String("summary", "", "")
		cmd.Flags().String("desc", "", "")
		cmd.Flags().String("repo", "", "")

        // Simulate flags
        cmd.Flags().Set("summary", "Test Task")
        cmd.Flags().Set("desc", "Description")
        cmd.Flags().Set("repo", "http://repo.com")

		err := runTaskAdd(cmd, []string{})
		assert.NoError(t, err)

		// Verify file content
		data, err := os.ReadFile(workFile)
		assert.NoError(t, err)
		var items []orchestrator.WorkItem
		err = json.Unmarshal(data, &items)
		assert.NoError(t, err)
		assert.Len(t, items, 1)
		assert.Equal(t, "Test Task", items[0].Summary)
		assert.Equal(t, "Description", items[0].Description)
		assert.Equal(t, "http://repo.com", items[0].RepoURL)
	})

	// Test List
	t.Run("List Tasks", func(t *testing.T) {
		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		err := runTaskList(&cobra.Command{}, []string{})
		assert.NoError(t, err)

		w.Close()
		os.Stdout = oldStdout

		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		output := buf.String()

		assert.Contains(t, output, "Test Task")
		assert.Contains(t, output, "http://repo.com")
	})

    // Test Clear
    t.Run("Clear Tasks", func(t *testing.T) {
        err := runTaskClear(&cobra.Command{}, []string{})
        assert.NoError(t, err)

        // Verify file empty
        data, err := os.ReadFile(workFile)
        assert.NoError(t, err)
        var items []orchestrator.WorkItem
        err = json.Unmarshal(data, &items)
        assert.NoError(t, err)
        assert.Len(t, items, 0)
    })
}
