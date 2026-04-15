package orchestrator

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestJanitor_Cleanup(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	client := new(MockDockerClient)
	ctx := context.Background()

	// Setup containers
	now := time.Now()
	oldTime := now.Add(-48 * time.Hour)
	newTime := now.Add(-1 * time.Hour)

	containers := []types.Container{
		{
			ID:      "old-container",
			Created: oldTime.Unix(),
			Labels: map[string]string{
				"created-by": "recac-orchestrator",
				"work-item":  "TASK-1",
			},
		},
		{
			ID:      "new-container",
			Created: newTime.Unix(),
			Labels: map[string]string{
				"created-by": "recac-orchestrator",
				"work-item":  "TASK-2",
			},
		},
		{
			ID:      "manual-container",
			Created: oldTime.Unix(),
			Labels: map[string]string{
				"created-by": "manual",
			},
		},
		{
			ID:      "exited-container",
			Created: newTime.Unix(),
			State:   "exited",
			Labels: map[string]string{
				"created-by": "recac-orchestrator",
				"work-item":  "TASK-3",
			},
		},
		{
			ID:      "dead-container",
			Created: newTime.Unix(),
			State:   "dead",
			Labels: map[string]string{
				"created-by": "recac-orchestrator",
				"work-item":  "TASK-4",
			},
		},
	}

	client.On("ListContainers", ctx, mock.MatchedBy(func(opts container.ListOptions) bool {
		return opts.All == true
	})).Return(containers, nil)

	// Expect removal of old-container, exited-container, and dead-container
	client.On("RemoveContainer", ctx, "old-container", true).Return(nil)
	client.On("RemoveContainer", ctx, "exited-container", true).Return(nil)
	client.On("RemoveContainer", ctx, "dead-container", true).Return(nil)

	// Janitor setup
	janitor := NewJanitor(logger, client, 1*time.Minute, 24*time.Hour, false, "")

	err := janitor.Cleanup(ctx)
	assert.NoError(t, err)

	client.AssertExpectations(t)
	// Ensure new-container and manual-container were NOT removed
	client.AssertNotCalled(t, "RemoveContainer", ctx, "new-container", mock.Anything)
	client.AssertNotCalled(t, "RemoveContainer", ctx, "manual-container", mock.Anything)
}

func TestJanitor_Cleanup_DryRun(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	client := new(MockDockerClient)
	ctx := context.Background()

	now := time.Now()
	oldTime := now.Add(-48 * time.Hour)

	containers := []types.Container{
		{
			ID:      "old-container",
			Created: oldTime.Unix(),
			Labels: map[string]string{
				"created-by": "recac-orchestrator",
			},
		},
	}

	client.On("ListContainers", ctx, mock.Anything).Return(containers, nil)

	// Janitor setup with dryRun=true
	janitor := NewJanitor(logger, client, 1*time.Minute, 24*time.Hour, true, "")

	err := janitor.Cleanup(ctx)
	assert.NoError(t, err)

	// Expect NO removal calls
	client.AssertNotCalled(t, "RemoveContainer")
}

func TestJanitor_Cleanup_ListError(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	client := new(MockDockerClient)
	ctx := context.Background()

	client.On("ListContainers", ctx, mock.Anything).Return([]types.Container{}, errors.New("list failed"))

	janitor := NewJanitor(logger, client, 1*time.Minute, 24*time.Hour, false, "")

	err := janitor.Cleanup(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "list failed")
}

func TestJanitor_Cleanup_Logs(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	ctx := context.Background()

	// Setup temp directory
	tempDir := t.TempDir()

	// Setup log files
	now := time.Now()
	oldTime := now.Add(-48 * time.Hour)
	newTime := now.Add(-1 * time.Hour)

	oldLogFile, err := os.CreateTemp(tempDir, "old-*.log.gz")
	assert.NoError(t, err)
	oldLogFile.Close()
	err = os.Chtimes(oldLogFile.Name(), oldTime, oldTime)
	assert.NoError(t, err)

	newLogFile, err := os.CreateTemp(tempDir, "new-*.log.gz")
	assert.NoError(t, err)
	newLogFile.Close()
	err = os.Chtimes(newLogFile.Name(), newTime, newTime)
	assert.NoError(t, err)

	// Create a non-log file that should be ignored
	otherFile, err := os.CreateTemp(tempDir, "other-*.txt")
	assert.NoError(t, err)
	otherFile.Close()
	err = os.Chtimes(otherFile.Name(), oldTime, oldTime)
	assert.NoError(t, err)

	// Janitor setup
	janitor := NewJanitor(logger, nil, 1*time.Minute, 24*time.Hour, false, tempDir)

	err = janitor.Cleanup(ctx)
	assert.NoError(t, err)

	// Verify old log file is deleted
	_, err = os.Stat(oldLogFile.Name())
	assert.True(t, os.IsNotExist(err))

	// Verify new log file is kept
	_, err = os.Stat(newLogFile.Name())
	assert.NoError(t, err)

	// Verify non-log file is kept
	_, err = os.Stat(otherFile.Name())
	assert.NoError(t, err)
}

func TestJanitor_Cleanup_Logs_RemoveError(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	ctx := context.Background()

	// Setup temp directory
	tempDir := t.TempDir()

	subDir := filepath.Join(tempDir, "sub")
	err := os.Mkdir(subDir, 0755)
	assert.NoError(t, err)

	now := time.Now()
	oldTime := now.Add(-48 * time.Hour)

	oldLogFile, err := os.CreateTemp(subDir, "old-*.log.gz")
	assert.NoError(t, err)
	oldLogFile.Close()
	err = os.Chtimes(oldLogFile.Name(), oldTime, oldTime)
	assert.NoError(t, err)

	// Janitor setup
	janitor := NewJanitor(logger, nil, 1*time.Minute, 24*time.Hour, false, subDir)

	// make subDir un-writable so Remove fails
	err = os.Chmod(subDir, 0555)
	assert.NoError(t, err)
	// Fix permissions
	defer os.Chmod(subDir, 0755)

	err = janitor.Cleanup(ctx)
	assert.NoError(t, err) // It continues and logs the error, doesn't return the error.

	// Check the file still exists
	_, err = os.Stat(oldLogFile.Name())
	assert.NoError(t, err)
}

func TestJanitor_Cleanup_Logs_ReadDirError(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	ctx := context.Background()

	janitor := NewJanitor(logger, nil, 1*time.Minute, 24*time.Hour, false, "/invalid/directory/path/does/not/exist")

	err := janitor.Cleanup(ctx)
	// IsNotExist returns nil
	assert.NoError(t, err)
}

func TestJanitor_Cleanup_Logs_ReadDirRealError(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	ctx := context.Background()

	// Setup temp directory
	tempDir := t.TempDir()

	subDir := filepath.Join(tempDir, "sub")
	err := os.Mkdir(subDir, 0755)
	assert.NoError(t, err)

	// we want os.ReadDir(subDir) to fail, so we give no permissions
	err = os.Chmod(subDir, 0000)
	assert.NoError(t, err)
	defer os.Chmod(subDir, 0755)

	janitor := NewJanitor(logger, nil, 1*time.Minute, 24*time.Hour, false, subDir)

	err = janitor.Cleanup(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read log directory")
}
