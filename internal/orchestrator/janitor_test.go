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
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	mockDocker := new(MockDockerClient)

	config := JanitorConfig{
		MaxAge:   1 * time.Hour,
		Interval: 10 * time.Minute,
		DryRun:   false,
	}

	janitor := NewJanitor(mockDocker, config, logger)

	// Case 1: Container is new, should NOT be removed
	newContainer := types.Container{
		ID:      "new-container",
		Created: time.Now().Unix(),
		Labels: map[string]string{
			"created-by": "recac-orchestrator",
			"created-at": time.Now().Format(time.RFC3339),
		},
	}

	// Case 2: Container is old, SHOULD be removed
	oldContainer := types.Container{
		ID:      "old-container-1234567890", // > 12 chars
		Created: time.Now().Add(-2 * time.Hour).Unix(),
		Labels: map[string]string{
			"created-by": "recac-orchestrator",
			"created-at": time.Now().Add(-2 * time.Hour).Format(time.RFC3339),
		},
	}

	// Case 3: Container without timestamp label, rely on Created field (Old)
	oldContainerNoLabel := types.Container{
		ID:      "old-no-label",
		Created: time.Now().Add(-3 * time.Hour).Unix(),
		Labels: map[string]string{
			"created-by": "recac-orchestrator",
		},
	}

	mockDocker.On("ListContainers", ctx, mock.MatchedBy(func(opts container.ListOptions) bool {
		return opts.All == true && opts.Filters.ExactMatch("label", "created-by=recac-orchestrator")
	})).Return([]types.Container{newContainer, oldContainer, oldContainerNoLabel}, nil)

	mockDocker.On("RemoveContainer", ctx, oldContainer.ID, true).Return(nil)
	mockDocker.On("RemoveContainer", ctx, oldContainerNoLabel.ID, true).Return(nil)

	err := janitor.cleanup(ctx)
	assert.NoError(t, err)

	mockDocker.AssertExpectations(t)
	mockDocker.AssertNotCalled(t, "RemoveContainer", ctx, newContainer.ID, mock.Anything)
}

func TestJanitor_Cleanup_DryRun(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	mockDocker := new(MockDockerClient)

	config := JanitorConfig{
		MaxAge:   1 * time.Hour,
		Interval: 10 * time.Minute,
		DryRun:   true,
	}

	janitor := NewJanitor(mockDocker, config, logger)

	oldContainer := types.Container{
		ID:      "old-container-123",
		Created: time.Now().Add(-2 * time.Hour).Unix(),
		Labels: map[string]string{
			"created-by": "recac-orchestrator",
			"created-at": time.Now().Add(-2 * time.Hour).Format(time.RFC3339),
		},
	}

	mockDocker.On("ListContainers", ctx, mock.Anything).Return([]types.Container{oldContainer}, nil)

	// RemoveContainer should NOT be called in dry run
	err := janitor.cleanup(ctx)
	assert.NoError(t, err)

	mockDocker.AssertNotCalled(t, "RemoveContainer")
}
