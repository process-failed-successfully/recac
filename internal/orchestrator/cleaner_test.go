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

func TestCleaner_Cleanup(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	mockDocker := new(MockDockerClient)
	cleaner := NewCleaner(logger, mockDocker)

	ctx := context.Background()
	maxAge := 1 * time.Hour
	now := time.Now()

	containers := []types.Container{
		{ID: "old-1", Created: now.Add(-2 * time.Hour).Unix(), Names: []string{"/old1"}},
		{ID: "new-1", Created: now.Add(-30 * time.Minute).Unix(), Names: []string{"/new1"}},
		{ID: "old-2", Created: now.Add(-3 * time.Hour).Unix(), Names: []string{"/old2"}},
	}

	// Expect ListContainers with correct filter
	mockDocker.On("ListContainers", ctx, mock.MatchedBy(func(opts container.ListOptions) bool {
		return opts.All == true && opts.Filters.ExactMatch("label", "created-by=recac-orchestrator")
	})).Return(containers, nil)

	// Expect RemoveContainer for old containers
	mockDocker.On("RemoveContainer", ctx, "old-1", true).Return(nil)
	mockDocker.On("RemoveContainer", ctx, "old-2", true).Return(nil)

	// Execute
	removed, err := cleaner.Cleanup(ctx, maxAge, false)

	assert.NoError(t, err)
	assert.Len(t, removed, 2)
	assert.Contains(t, removed, "old-1")
	assert.Contains(t, removed, "old-2")
	assert.NotContains(t, removed, "new-1")

	mockDocker.AssertExpectations(t)
	mockDocker.AssertNotCalled(t, "RemoveContainer", ctx, "new-1", true)
}

func TestCleaner_Cleanup_DryRun(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	mockDocker := new(MockDockerClient)
	cleaner := NewCleaner(logger, mockDocker)

	ctx := context.Background()
	maxAge := 1 * time.Hour
	now := time.Now()

	containers := []types.Container{
		{ID: "old-1", Created: now.Add(-2 * time.Hour).Unix()},
	}

	mockDocker.On("ListContainers", ctx, mock.Anything).Return(containers, nil)

	// Execute with dryRun=true
	removed, err := cleaner.Cleanup(ctx, maxAge, true)

	assert.NoError(t, err)
	assert.Len(t, removed, 1)
	assert.Contains(t, removed, "old-1")

	// Verify RemoveContainer was NOT called
	mockDocker.AssertNotCalled(t, "RemoveContainer")
}

func TestCleaner_Cleanup_ListError(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	mockDocker := new(MockDockerClient)
	cleaner := NewCleaner(logger, mockDocker)

	mockDocker.On("ListContainers", mock.Anything, mock.Anything).Return([]types.Container{}, errors.New("list failed"))

	_, err := cleaner.Cleanup(context.Background(), time.Hour, false)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "list failed")
}
