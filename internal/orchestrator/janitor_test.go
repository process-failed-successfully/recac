package orchestrator

import (
	"context"
	"errors"
	"io"
	"log/slog"
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
	janitor := NewJanitor(logger, client, 1*time.Minute, 24*time.Hour, false)

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
	janitor := NewJanitor(logger, client, 1*time.Minute, 24*time.Hour, true)

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

	janitor := NewJanitor(logger, client, 1*time.Minute, 24*time.Hour, false)

	err := janitor.Cleanup(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "list failed")
}
