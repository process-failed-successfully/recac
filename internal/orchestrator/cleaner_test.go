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
	mockDocker := new(MockDockerClient)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cleaner := NewCleaner(logger, mockDocker, "test-project")

	now := time.Now()
	oldTime := now.Add(-2 * time.Hour).Unix()
	newTime := now.Add(-10 * time.Minute).Unix()

	containers := []types.Container{
		{ID: "old-container", Created: oldTime, Status: "Exited"},
		{ID: "new-container", Created: newTime, Status: "Up"},
	}

	mockDocker.On("ListContainers", mock.Anything, mock.MatchedBy(func(opts container.ListOptions) bool {
		return opts.All == true && opts.Filters.ExactMatch("label", "created-by=test-project")
	})).Return(containers, nil)

	mockDocker.On("RemoveContainer", mock.Anything, "old-container", true).Return(nil)

	// Expect new-container NOT to be removed
	// But we don't mock it, so if it's called, mock will fail (unexpected call)

	removed, err := cleaner.Cleanup(context.Background(), 1*time.Hour)

	assert.NoError(t, err)
	assert.Len(t, removed, 1)
	assert.Equal(t, "old-container", removed[0])

	mockDocker.AssertExpectations(t)
}

func TestCleaner_Cleanup_ListError(t *testing.T) {
	mockDocker := new(MockDockerClient)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cleaner := NewCleaner(logger, mockDocker, "test-project")

	mockDocker.On("ListContainers", mock.Anything, mock.Anything).Return(([]types.Container)(nil), errors.New("list failed"))

	removed, err := cleaner.Cleanup(context.Background(), 1*time.Hour)

	assert.Error(t, err)
	assert.Nil(t, removed)
	assert.Contains(t, err.Error(), "list failed")
}
