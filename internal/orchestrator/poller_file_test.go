package orchestrator

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFilePoller(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "poller_test")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	workFile := filepath.Join(tmpDir, "work.json")

	t.Run("New FilePoller", func(t *testing.T) {
		poller := NewFilePoller(workFile)
		assert.NotNil(t, poller)
	})

	t.Run("Poll Empty/Missing File", func(t *testing.T) {
		poller := NewFilePoller(workFile)
		items, err := poller.Poll(context.Background(), silentLogger)
		assert.NoError(t, err)
		assert.Nil(t, items)
	})

	t.Run("Poll With Items", func(t *testing.T) {
		content := `[{"id": "TASK-1", "summary": "Task 1"}]`
		os.WriteFile(workFile, []byte(content), 0644)

		poller := NewFilePoller(workFile)
		items, err := poller.Poll(context.Background(), silentLogger)
		assert.NoError(t, err)
		assert.Len(t, items, 1)
		assert.Equal(t, "TASK-1", items[0].ID)

		// Poll again should be empty, as Poll now claims the items
		items2, err := poller.Poll(context.Background(), silentLogger)
		assert.NoError(t, err)
		assert.Len(t, items2, 0)
	})

	t.Run("Update Status", func(t *testing.T) {
		poller := NewFilePoller(workFile)
		err := poller.UpdateStatus(context.Background(), WorkItem{ID: "TASK-1"}, "done", "comment")
		assert.NoError(t, err)
	})
}

func TestFilePoller_Poll_BadJSON(t *testing.T) {
	tempFile, err := os.CreateTemp("", "bad-json-*.json")
	require.NoError(t, err)
	defer os.Remove(tempFile.Name())

	_, err = tempFile.Write([]byte("{invalid json}"))
	require.NoError(t, err)
	tempFile.Close()

	poller := NewFilePoller(tempFile.Name())
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	_, err = poller.Poll(context.Background(), logger)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to unmarshal")
}

func TestFilePoller_Ping_DirInsteadOfFile(t *testing.T) {
	tempDir := t.TempDir()
	poller := NewFilePoller(tempDir)
	err := poller.Ping(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "watch path is a directory")
}
