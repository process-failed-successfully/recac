package orchestrator

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFilePoller_Poll(t *testing.T) {
	tempDir := t.TempDir()
	workFilePath := filepath.Join(tempDir, "work.json")

	items := []WorkItem{
		{ID: "FILE-1", Summary: "File task 1"},
		{ID: "FILE-2", Summary: "File task 2"},
	}
	data, err := json.Marshal(items)
	require.NoError(t, err)
	err = os.WriteFile(workFilePath, data, 0644)
	require.NoError(t, err)

	poller := NewFilePoller(workFilePath)

	// First poll should return all items
	polledItems, err := poller.Poll(context.Background())
	require.NoError(t, err)
	assert.Len(t, polledItems, 2)

	// Claim one item
	err = poller.Claim(context.Background(), items[0])
	require.NoError(t, err)

	// Second poll should return only the unclaimed item
	polledItems, err = poller.Poll(context.Background())
	require.NoError(t, err)
	require.Len(t, polledItems, 1)
	assert.Equal(t, "FILE-2", polledItems[0].ID)
}

func TestFilePoller_Poll_NoFile(t *testing.T) {
	tempDir := t.TempDir()
	workFilePath := filepath.Join(tempDir, "nonexistent.json")

	poller := NewFilePoller(workFilePath)

	polledItems, err := poller.Poll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, polledItems)
}

func TestFilePoller_Poll_MalformedJSON(t *testing.T) {
	tempDir := t.TempDir()
	workFilePath := filepath.Join(tempDir, "work.json")

	err := os.WriteFile(workFilePath, []byte("{not json}"), 0644)
	require.NoError(t, err)

	poller := NewFilePoller(workFilePath)

	_, err = poller.Poll(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to unmarshal work items")
}

func TestFilePoller_UpdateStatus(t *testing.T) {
	poller := NewFilePoller("")
	err := poller.UpdateStatus(context.Background(), WorkItem{ID: "TEST-1"}, "Completed", "All done")
	assert.NoError(t, err, "UpdateStatus should be a no-op and not return an error")
}
