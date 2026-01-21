package orchestrator

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFileDirPoller(t *testing.T) {
	// Setup a temporary directory for the test
	tmpDir, err := os.MkdirTemp("", "file-dir-poller-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create a logger
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// Create the poller
	poller, err := NewFileDirPoller(tmpDir)
	require.NoError(t, err)

	// Test case 1: No files in the directory
	items, err := poller.Poll(context.Background(), logger)
	assert.NoError(t, err)
	assert.Empty(t, items)

	// Test case 2: One valid JSON file
	task1 := WorkItem{ID: "task-1", Summary: "Test Task 1"}
	task1Data, err := json.Marshal(task1)
	require.NoError(t, err)
	task1Path := filepath.Join(tmpDir, "task1.json")
	err = os.WriteFile(task1Path, task1Data, 0644)
	require.NoError(t, err)

	items, err = poller.Poll(context.Background(), logger)
	assert.NoError(t, err)
	assert.Len(t, items, 1)
	assert.Equal(t, "task-1", items[0].ID)

	// Verify the file was moved
	_, err = os.Stat(task1Path)
	assert.True(t, os.IsNotExist(err))
	processedPath := filepath.Join(tmpDir, "processed", "task1.json")
	_, err = os.Stat(processedPath)
	assert.NoError(t, err)

	// Test case 3: Poll again, should be no new items
	items, err = poller.Poll(context.Background(), logger)
	assert.NoError(t, err)
	assert.Empty(t, items)

	// Test case 4: Two new files, one invalid
	task2 := WorkItem{ID: "task-2", Summary: "Test Task 2"}
	task2Data, err := json.Marshal(task2)
	require.NoError(t, err)
	task2Path := filepath.Join(tmpDir, "task2.json")
	err = os.WriteFile(task2Path, task2Data, 0644)
	require.NoError(t, err)

	invalidPath := filepath.Join(tmpDir, "invalid.txt")
	err = os.WriteFile(invalidPath, []byte("this is not json"), 0644)
	require.NoError(t, err)

	malformedPath := filepath.Join(tmpDir, "malformed.json")
	err = os.WriteFile(malformedPath, []byte("this is not json"), 0644)
	require.NoError(t, err)

	items, err = poller.Poll(context.Background(), logger)
	assert.NoError(t, err)
	assert.Len(t, items, 1)
	assert.Equal(t, "task-2", items[0].ID)

	// Verify files were moved/left alone
	_, err = os.Stat(task2Path)
	assert.True(t, os.IsNotExist(err))
	_, err = os.Stat(filepath.Join(tmpDir, "processed", "task2.json"))
	assert.NoError(t, err)
	_, err = os.Stat(invalidPath)
	assert.NoError(t, err)
	_, err = os.Stat(malformedPath)
	assert.NoError(t, err) // Malformed json is not moved
}

func TestFileDirPoller_UpdateStatus(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "file-dir-update-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	poller, err := NewFileDirPoller(tmpDir)
	require.NoError(t, err)

	item := WorkItem{ID: "task-1"}
	err = poller.UpdateStatus(context.Background(), item, "done", "completed")
	assert.NoError(t, err)
}
