package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"recac/internal/orchestrator"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSubmitToFileDir(t *testing.T) {
	tmpDir := t.TempDir()

	item := orchestrator.WorkItem{
		ID:          "TEST-1",
		Summary:     "Test Summary",
		Description: "Test Description",
	}

	err := submitToFileDir(tmpDir, item)
	require.NoError(t, err)

	// Verify file exists
	path := filepath.Join(tmpDir, "TEST-1.json")
	require.FileExists(t, path)

	// Verify content
	data, err := os.ReadFile(path)
	require.NoError(t, err)

	var readItem orchestrator.WorkItem
	err = json.Unmarshal(data, &readItem)
	require.NoError(t, err)

	assert.Equal(t, item.ID, readItem.ID)
	assert.Equal(t, item.Summary, readItem.Summary)
}

func TestSubmitToFile(t *testing.T) {
	tmpDir := t.TempDir()
	workFile := filepath.Join(tmpDir, "work_items.json")

	item1 := orchestrator.WorkItem{ID: "TEST-1", Summary: "Sum 1"}
	item2 := orchestrator.WorkItem{ID: "TEST-2", Summary: "Sum 2"}

	// 1. Submit first item
	err := submitToFile(workFile, item1)
	require.NoError(t, err)

	// Verify
	data, err := os.ReadFile(workFile)
	require.NoError(t, err)
	var items []orchestrator.WorkItem
	json.Unmarshal(data, &items)
	assert.Len(t, items, 1)
	assert.Equal(t, "TEST-1", items[0].ID)

	// 2. Submit second item
	err = submitToFile(workFile, item2)
	require.NoError(t, err)

	// Verify append
	data, err = os.ReadFile(workFile)
	require.NoError(t, err)
	json.Unmarshal(data, &items)
	assert.Len(t, items, 2)
	assert.Equal(t, "TEST-1", items[0].ID)
	assert.Equal(t, "TEST-2", items[1].ID)
}
