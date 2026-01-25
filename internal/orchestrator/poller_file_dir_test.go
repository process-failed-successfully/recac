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

func TestFileDirPoller_New(t *testing.T) {
	tempDir := t.TempDir()
	poller, err := NewFileDirPoller(tempDir)
	require.NoError(t, err)
	assert.NotNil(t, poller)

	// Check if processed directory was created
	processedDir := filepath.Join(tempDir, "processed")
	info, err := os.Stat(processedDir)
	assert.NoError(t, err)
	assert.True(t, info.IsDir())
}

func TestFileDirPoller_Poll(t *testing.T) {
	tempDir := t.TempDir()
	processedDir := filepath.Join(tempDir, "processed")
	require.NoError(t, os.MkdirAll(processedDir, 0755))

	poller, err := NewFileDirPoller(tempDir)
	require.NoError(t, err)

	// Create a valid work item file
	item := WorkItem{
		ID:          "task-1",
		Summary:     "Test Task",
		Description: "Do something",
		EnvVars:     map[string]string{"foo": "bar"},
	}
	itemData, err := json.Marshal(item)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tempDir, "task1.json"), itemData, 0644)
	require.NoError(t, err)

	// Create an invalid JSON file
	err = os.WriteFile(filepath.Join(tempDir, "invalid.json"), []byte("{invalid"), 0644)
	require.NoError(t, err)

	// Create a non-JSON file
	err = os.WriteFile(filepath.Join(tempDir, "other.txt"), []byte("text"), 0644)
	require.NoError(t, err)

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	ctx := context.Background()

	// Poll
	items, err := poller.Poll(ctx, logger)
	require.NoError(t, err)

	// Verify we got 1 valid item
	assert.Len(t, items, 1)
	assert.Equal(t, "task-1", items[0].ID)
	assert.Equal(t, "Test Task", items[0].Summary)

	// Verify file movement
	// task1.json should be in processed
	_, err = os.Stat(filepath.Join(processedDir, "task1.json"))
	assert.NoError(t, err)
	_, err = os.Stat(filepath.Join(tempDir, "task1.json"))
	assert.True(t, os.IsNotExist(err))

	// invalid.json should remain
	_, err = os.Stat(filepath.Join(tempDir, "invalid.json"))
	assert.NoError(t, err)

	// other.txt should remain
	_, err = os.Stat(filepath.Join(tempDir, "other.txt"))
	assert.NoError(t, err)
}

func TestFileDirPoller_UpdateStatus(t *testing.T) {
	tempDir := t.TempDir()
	poller, _ := NewFileDirPoller(tempDir)

	item := WorkItem{ID: "task-1"}
	err := poller.UpdateStatus(context.Background(), item, "completed", "done")
	assert.NoError(t, err)
}

func TestFileDirPoller_Poll_ReadDirError(t *testing.T) {
	// Use a non-existent directory to force error
	poller := &FileDirPoller{
		watchDir: "/path/to/non/existent/dir",
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	_, err := poller.Poll(context.Background(), logger)
	assert.Error(t, err)
}
