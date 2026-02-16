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

func TestCleanupOrphanedContainers(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	ctx := context.Background()

	t.Run("Removes all orphaned containers", func(t *testing.T) {
		mockDocker := new(MockDockerClient)

		containers := []types.Container{
			{ID: "c1", Names: []string{"/recac-1"}, Created: time.Now().Unix()},
			{ID: "c2", Names: []string{"/recac-2"}, Created: time.Now().Add(-2 * time.Hour).Unix()},
		}

		mockDocker.On("ListContainers", ctx, mock.MatchedBy(func(opts container.ListOptions) bool {
			if !opts.All {
				return false
			}
			values := opts.Filters.Get("label")
			for _, v := range values {
				if v == "created-by=recac-orchestrator" {
					return true
				}
			}
			return false
		})).Return(containers, nil)

		mockDocker.On("RemoveContainer", ctx, "c1", true).Return(nil)
		mockDocker.On("RemoveContainer", ctx, "c2", true).Return(nil)

		err := CleanupOrphanedContainers(ctx, mockDocker, logger, 0)
		assert.NoError(t, err)

		mockDocker.AssertExpectations(t)
	})

	t.Run("Removes only old containers", func(t *testing.T) {
		mockDocker := new(MockDockerClient)

		now := time.Now()
		containers := []types.Container{
			{ID: "new", Created: now.Add(-10 * time.Minute).Unix()}, // 10 mins old
			{ID: "old", Created: now.Add(-2 * time.Hour).Unix()},    // 2 hours old
		}

		mockDocker.On("ListContainers", ctx, mock.Anything).Return(containers, nil)

		// Expect only "old" to be removed if threshold is 1 hour
		mockDocker.On("RemoveContainer", ctx, "old", true).Return(nil)

		err := CleanupOrphanedContainers(ctx, mockDocker, logger, 1*time.Hour)
		assert.NoError(t, err)

		mockDocker.AssertNotCalled(t, "RemoveContainer", ctx, "new", true)
		mockDocker.AssertExpectations(t)
	})

	t.Run("Handles ListContainers error", func(t *testing.T) {
		mockDocker := new(MockDockerClient)
		mockDocker.On("ListContainers", ctx, mock.Anything).Return([]types.Container{}, errors.New("list failed"))

		err := CleanupOrphanedContainers(ctx, mockDocker, logger, 0)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "list failed")
	})

	t.Run("Continues on RemoveContainer error", func(t *testing.T) {
		mockDocker := new(MockDockerClient)
		containers := []types.Container{{ID: "c1"}}

		mockDocker.On("ListContainers", ctx, mock.Anything).Return(containers, nil)
		mockDocker.On("RemoveContainer", ctx, "c1", true).Return(errors.New("remove failed"))

		// Should not return error, just log it
		err := CleanupOrphanedContainers(ctx, mockDocker, logger, 0)
		assert.NoError(t, err)

		mockDocker.AssertExpectations(t)
	})
}
