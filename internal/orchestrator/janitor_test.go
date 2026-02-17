package orchestrator

import (
	"context"
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
	mockDocker := new(MockDockerClient)
	janitor := NewJanitor(logger, mockDocker, time.Hour, time.Hour, false)

	ctx := context.Background()
	now := time.Now()

	// Setup mock containers
	oldContainer := types.Container{
		ID:      "old-container",
		Created: now.Add(-2 * time.Hour).Unix(),
		Labels:  map[string]string{"created-by": "recac-orchestrator"},
	}
	newContainer := types.Container{
		ID:      "new-container",
		Created: now.Add(-30 * time.Minute).Unix(),
		Labels:  map[string]string{"created-by": "recac-orchestrator"},
	}

	// Expect ListContainers with specific filter
	mockDocker.On("ListContainers", ctx, mock.MatchedBy(func(opts container.ListOptions) bool {
		return opts.All == true && opts.Filters.ExactMatch("label", "created-by=recac-orchestrator")
	})).Return([]types.Container{oldContainer, newContainer}, nil)

	// Expect RemoveContainer for old container only
	mockDocker.On("RemoveContainer", ctx, "old-container", true).Return(nil)

	err := janitor.Cleanup(ctx)
	assert.NoError(t, err)

	mockDocker.AssertExpectations(t)
	mockDocker.AssertNotCalled(t, "RemoveContainer", ctx, "new-container", mock.Anything)
}

func TestJanitor_Cleanup_DryRun(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	mockDocker := new(MockDockerClient)
	janitor := NewJanitor(logger, mockDocker, time.Hour, time.Hour, true) // DryRun = true

	ctx := context.Background()
	now := time.Now()

	oldContainer := types.Container{
		ID:      "old-container",
		Created: now.Add(-2 * time.Hour).Unix(),
	}

	mockDocker.On("ListContainers", ctx, mock.Anything).Return([]types.Container{oldContainer}, nil)

	// Expect NO RemoveContainer calls
	err := janitor.Cleanup(ctx)
	assert.NoError(t, err)

	mockDocker.AssertNotCalled(t, "RemoveContainer", mock.Anything, mock.Anything, mock.Anything)
}
