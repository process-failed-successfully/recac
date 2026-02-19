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

	// ListContainers returns only matching containers because we use filters
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
	}

	client.On("ListContainers", ctx, mock.MatchedBy(func(opts container.ListOptions) bool {
		filters := opts.Filters
		if filters.Len() == 0 {
			return false
		}
		hasLabel := false
		for _, f := range filters.Get("label") {
			if f == "created-by=recac-orchestrator" {
				hasLabel = true
				break
			}
		}
		return opts.All == true && hasLabel
	})).Return(containers, nil)

	// Expect removal of old-container
	client.On("RemoveContainer", ctx, "old-container", true).Return(nil)

	// Janitor setup
	janitor := NewJanitor(logger, client, 1*time.Minute, 24*time.Hour, false)

	err := janitor.Cleanup(ctx)
	assert.NoError(t, err)

	client.AssertExpectations(t)
	// Ensure new-container was NOT removed
	client.AssertNotCalled(t, "RemoveContainer", ctx, "new-container", mock.Anything)
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
