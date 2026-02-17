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

	janitor := NewJanitor(mockDocker, logger, false)

	now := time.Now()
	expired := now.Add(-2 * time.Hour)
	active := now.Add(-30 * time.Minute)

	containers := []types.Container{
		{
			ID:      "expired-container",
			Created: expired.Unix(),
		},
		{
			ID:      "active-container",
			Created: active.Unix(),
		},
	}

	// Verify filters
	mockDocker.On("ListContainers", ctx, mock.MatchedBy(func(opts container.ListOptions) bool {
		return opts.All == true && opts.Filters.ExactMatch("label", "created-by=recac-orchestrator")
	})).Return(containers, nil)

	// Expect remove only for expired container
	mockDocker.On("RemoveContainer", ctx, "expired-container", true).Return(nil)

	err := janitor.Cleanup(ctx, 1*time.Hour)
	assert.NoError(t, err)

	mockDocker.AssertExpectations(t)
	mockDocker.AssertNotCalled(t, "RemoveContainer", ctx, "active-container", mock.Anything)
}

func TestJanitor_DryRun(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	mockDocker := new(MockDockerClient)

	// Dry run enabled
	janitor := NewJanitor(mockDocker, logger, true)

	now := time.Now()
	expired := now.Add(-2 * time.Hour)

	containers := []types.Container{
		{
			ID:      "expired-container",
			Created: expired.Unix(),
		},
	}

	mockDocker.On("ListContainers", ctx, mock.Anything).Return(containers, nil)

	// Expect NO remove calls
	err := janitor.Cleanup(ctx, 1*time.Hour)
	assert.NoError(t, err)

	mockDocker.AssertNotCalled(t, "RemoveContainer")
}
