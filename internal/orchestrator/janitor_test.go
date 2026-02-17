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

	janitor := NewJanitor(logger, mockDocker, 24*time.Hour, 1*time.Minute, false)

	// Test Case 1: Old container -> Remove
	oldTime := time.Now().Add(-25 * time.Hour)
	oldContainer := types.Container{
		ID: "old-container",
		Labels: map[string]string{
			"created-by": "recac-orchestrator",
			"created-at": oldTime.Format(time.RFC3339),
		},
		State: "exited",
	}

	// Test Case 2: New container -> Keep
	newTime := time.Now().Add(-1 * time.Hour)
	newContainer := types.Container{
		ID: "new-container",
		Labels: map[string]string{
			"created-by": "recac-orchestrator",
			"created-at": newTime.Format(time.RFC3339),
		},
		State: "running",
	}

	// Test Case 3: Container without timestamp label but old creation time -> Remove (Fallback)
	oldCreationTime := time.Now().Add(-26 * time.Hour).Unix()
	fallbackContainer := types.Container{
		ID: "fallback-container",
		Labels: map[string]string{
			"created-by": "recac-orchestrator",
		},
		Created: oldCreationTime,
		State:   "exited",
	}

	mockDocker.On("ListContainers", mock.Anything, mock.MatchedBy(func(opts container.ListOptions) bool {
		// Just check that All is true and we have some filters
		return opts.All == true && opts.Filters.Len() > 0
	})).Return([]types.Container{oldContainer, newContainer, fallbackContainer}, nil)

	mockDocker.On("RemoveContainer", mock.Anything, "old-container", true).Return(nil)
	mockDocker.On("RemoveContainer", mock.Anything, "fallback-container", true).Return(nil)

	janitor.Cleanup(context.Background())

	mockDocker.AssertExpectations(t)
	mockDocker.AssertNotCalled(t, "RemoveContainer", mock.Anything, "new-container", mock.Anything)
}

func TestJanitor_Cleanup_DryRun(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	mockDocker := new(MockDockerClient)

	// Enable DryRun
	janitor := NewJanitor(logger, mockDocker, 24*time.Hour, 1*time.Minute, true)

	oldTime := time.Now().Add(-25 * time.Hour)
	oldContainer := types.Container{
		ID: "old-container",
		Labels: map[string]string{
			"created-by": "recac-orchestrator",
			"created-at": oldTime.Format(time.RFC3339),
		},
	}

	mockDocker.On("ListContainers", mock.Anything, mock.Anything).Return([]types.Container{oldContainer}, nil)

	// Should NOT call RemoveContainer
	janitor.Cleanup(context.Background())

	mockDocker.AssertExpectations(t)
	mockDocker.AssertNotCalled(t, "RemoveContainer", mock.Anything, mock.Anything, mock.Anything)
}

func TestJanitor_New(t *testing.T) {
    logger := slog.New(slog.NewTextHandler(io.Discard, nil))
    mockDocker := new(MockDockerClient)
    j := NewJanitor(logger, mockDocker, time.Hour, time.Minute, true)
    assert.NotNil(t, j)
    assert.Equal(t, time.Hour, j.MaxAge)
    assert.True(t, j.DryRun)
}
