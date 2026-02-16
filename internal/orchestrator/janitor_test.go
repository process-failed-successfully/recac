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
	mockDocker := new(MockDockerClient)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	janitor := NewJanitor(logger, mockDocker, "test-project", 1*time.Hour, 1*time.Minute)

	ctx := context.Background()
	now := time.Now()

	containers := []types.Container{
		{
			ID:      "old-exited-1234567890",
			Created: now.Add(-2 * time.Hour).Unix(),
			State:   "exited",
		},
		{
			ID:      "young-exited-1234567890",
			Created: now.Add(-30 * time.Minute).Unix(),
			State:   "exited",
		},
		{
			ID:      "old-running-1234567890",
			Created: now.Add(-2 * time.Hour).Unix(),
			State:   "running",
		},
		{
			ID:      "young-running-1234567890",
			Created: now.Add(-30 * time.Minute).Unix(),
			State:   "running",
		},
	}

	// Mock ListContainers
	mockDocker.On("ListContainers", ctx, mock.MatchedBy(func(opts container.ListOptions) bool {
		// Verify filters
		labels := opts.Filters.Get("label")
		for _, l := range labels {
			if l == "created-by=test-project" {
				return true
			}
		}
		return false
	})).Return(containers, nil)

	// Mock RemoveContainer
	// Should remove "old-exited" and "old-running"
	mockDocker.On("RemoveContainer", ctx, "old-exited-1234567890", true).Return(nil)
	mockDocker.On("RemoveContainer", ctx, "old-running-1234567890", true).Return(nil)

	err := janitor.Cleanup(ctx)
	assert.NoError(t, err)

	mockDocker.AssertCalled(t, "RemoveContainer", ctx, "old-exited-1234567890", true)
	mockDocker.AssertCalled(t, "RemoveContainer", ctx, "old-running-1234567890", true)
	mockDocker.AssertNotCalled(t, "RemoveContainer", ctx, "young-exited-1234567890", mock.Anything)
	mockDocker.AssertNotCalled(t, "RemoveContainer", ctx, "young-running-1234567890", mock.Anything)
}
