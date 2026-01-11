package orchestrator

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
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
		items, err := poller.Poll(context.Background())
		assert.NoError(t, err)
		assert.Nil(t, items)
	})

	t.Run("Poll With Items", func(t *testing.T) {
		content := `[{"id": "TASK-1", "summary": "Task 1"}]`
		os.WriteFile(workFile, []byte(content), 0644)

		poller := NewFilePoller(workFile)
		items, err := poller.Poll(context.Background())
		assert.NoError(t, err)
		assert.Len(t, items, 1)
		assert.Equal(t, "TASK-1", items[0].ID)

		// Claim
		err = poller.Claim(context.Background(), items[0])
		assert.NoError(t, err)

		// Poll again should be empty
		items2, err := poller.Poll(context.Background())
		assert.NoError(t, err)
		assert.Len(t, items2, 0)
	})

	t.Run("Update Status", func(t *testing.T) {
		poller := NewFilePoller(workFile)
		err := poller.UpdateStatus(context.Background(), WorkItem{ID: "TASK-1"}, "done", "comment")
		assert.NoError(t, err)
	})
}
