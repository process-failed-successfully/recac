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
	mockClient := new(MockDockerClient)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	maxAge := 1 * time.Hour

	janitor := NewJanitor(mockClient, logger, 10*time.Minute, maxAge, false)

	// Mock ListContainers
	now := time.Now()
	expiredContainer := types.Container{
		ID:      "expired-1234567890",
		Created: now.Add(-2 * time.Hour).Unix(), // 2 hours old
		Names:   []string{"/expired"},
	}
	freshContainer := types.Container{
		ID:      "fresh-1234567890",
		Created: now.Add(-30 * time.Minute).Unix(), // 30 mins old
		Names:   []string{"/fresh"},
	}

	mockClient.On("ListContainers", mock.Anything, mock.MatchedBy(func(opts container.ListOptions) bool {
		// Check for filter
		return opts.All == true && opts.Filters.ExactMatch("label", "created-by=recac-orchestrator")
	})).Return([]types.Container{expiredContainer, freshContainer}, nil)

	// Mock RemoveContainer
	// Should only be called for expired container
	mockClient.On("RemoveContainer", mock.Anything, "expired-1234567890", true).Return(nil)

	err := janitor.Cleanup(context.Background())
	assert.NoError(t, err)

	mockClient.AssertExpectations(t)
	mockClient.AssertNotCalled(t, "RemoveContainer", mock.Anything, "fresh-1234567890", mock.Anything)
}

func TestJanitor_Cleanup_DryRun(t *testing.T) {
	mockClient := new(MockDockerClient)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	maxAge := 1 * time.Hour

	janitor := NewJanitor(mockClient, logger, 10*time.Minute, maxAge, true) // DryRun = true

	// Mock ListContainers
	now := time.Now()
	expiredContainer := types.Container{
		ID:      "expired-1234567890",
		Created: now.Add(-2 * time.Hour).Unix(),
		Names:   []string{"/expired"},
	}

	mockClient.On("ListContainers", mock.Anything, mock.Anything).Return([]types.Container{expiredContainer}, nil)

	// RemoveContainer should NOT be called
	err := janitor.Cleanup(context.Background())
	assert.NoError(t, err)

	mockClient.AssertNotCalled(t, "RemoveContainer")
}

func TestJanitor_Cleanup_ListError(t *testing.T) {
	mockClient := new(MockDockerClient)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	janitor := NewJanitor(mockClient, logger, 10*time.Minute, 1*time.Hour, false)

	mockClient.On("ListContainers", mock.Anything, mock.Anything).Return([]types.Container{}, errors.New("list error"))

	err := janitor.Cleanup(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to list containers")
}
