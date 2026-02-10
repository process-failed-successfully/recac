package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"recac/internal/orchestrator"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTaskAdd_FilePoller(t *testing.T) {
	tmpDir := t.TempDir()
	workFile := filepath.Join(tmpDir, "work_items.json")

	// 1. Add first task
	cmd := taskAddCmd
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	// Reset flags (cobra commands persist flags)
	cmd.Flags().Set("summary", "Task 1")
	cmd.Flags().Set("poller", "file")
	cmd.Flags().Set("file", workFile)
	cmd.Flags().Set("repo", "http://github.com/org/repo")
	cmd.Flags().Set("env", "KEY=VALUE")

	err := runTaskAdd(cmd, []string{})
	require.NoError(t, err)

	assert.Contains(t, buf.String(), "Added task")
	assert.Contains(t, buf.String(), workFile)

	// Verify file content
	data, err := os.ReadFile(workFile)
	require.NoError(t, err)

	var items []orchestrator.WorkItem
	err = json.Unmarshal(data, &items)
	require.NoError(t, err)
	assert.Len(t, items, 1)
	assert.Equal(t, "Task 1", items[0].Summary)
	assert.Equal(t, "http://github.com/org/repo", items[0].RepoURL)
	assert.Equal(t, "VALUE", items[0].EnvVars["KEY"])

	// 2. Add second task
	buf.Reset()
	cmd.Flags().Set("summary", "Task 2")

	err = runTaskAdd(cmd, []string{})
	require.NoError(t, err)

	// Verify file content again
	data, err = os.ReadFile(workFile)
	require.NoError(t, err)

	err = json.Unmarshal(data, &items)
	require.NoError(t, err)
	assert.Len(t, items, 2)
	assert.Equal(t, "Task 1", items[0].Summary)
	assert.Equal(t, "Task 2", items[1].Summary)
}

func TestTaskAdd_FileDirPoller(t *testing.T) {
	tmpDir := t.TempDir()

	cmd := taskAddCmd
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	cmd.Flags().Set("summary", "Dir Task")
	cmd.Flags().Set("poller", "file-dir")
	cmd.Flags().Set("dir", tmpDir)
	cmd.Flags().Set("repo", "")
	cmd.Flags().Set("env", "")

	err := runTaskAdd(cmd, []string{})
	require.NoError(t, err)

	assert.Contains(t, buf.String(), "Added task")
	assert.Contains(t, buf.String(), tmpDir)

	// Verify file creation
	entries, err := os.ReadDir(tmpDir)
	require.NoError(t, err)
	assert.Len(t, entries, 1)
	assert.Contains(t, entries[0].Name(), "task-")
	assert.Contains(t, entries[0].Name(), ".json")

	// Verify content
	data, err := os.ReadFile(filepath.Join(tmpDir, entries[0].Name()))
	require.NoError(t, err)

	var item orchestrator.WorkItem
	err = json.Unmarshal(data, &item)
	require.NoError(t, err)
	assert.Equal(t, "Dir Task", item.Summary)
}

func TestTaskList_FilePoller(t *testing.T) {
	tmpDir := t.TempDir()
	workFile := filepath.Join(tmpDir, "work_items.json")

	// Create dummy file
	items := []orchestrator.WorkItem{
		{ID: "1234567890", Summary: "Existing Task", RepoURL: "repo-url"},
	}
	data, _ := json.Marshal(items)
	os.WriteFile(workFile, data, 0644)

	cmd := taskListCmd
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	cmd.Flags().Set("poller", "file")
	cmd.Flags().Set("file", workFile)

	err := runTaskList(cmd, []string{})
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "ID")
	assert.Contains(t, output, "SUMMARY")
	assert.Contains(t, output, "12345678") // Truncated ID
	assert.Contains(t, output, "Existing Task")
	assert.Contains(t, output, "repo-url")
}

func TestTaskList_FileDirPoller(t *testing.T) {
	tmpDir := t.TempDir()

	// Create dummy file
	item := orchestrator.WorkItem{
		ID: "abcdef1234", Summary: "Dir Task 1",
	}
	data, _ := json.Marshal(item)
	os.WriteFile(filepath.Join(tmpDir, "task-1.json"), data, 0644)

	cmd := taskListCmd
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	cmd.Flags().Set("poller", "file-dir")
	cmd.Flags().Set("dir", tmpDir)

	err := runTaskList(cmd, []string{})
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "abcdef12")
	assert.Contains(t, output, "Dir Task 1")
}
