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
	// Setup
	mockDocker := new(MockDockerClient)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	janitor := NewJanitor(mockDocker, 1*time.Hour, 10*time.Minute, logger)
	ctx := context.Background()
	now := time.Now().Unix()

	// Mock Containers
	containers := []types.Container{
		{
			ID:      "old-container-123", // Must be >= 12 chars
			Names:   []string{"/recac-agent-OLD"},
			Created: now - 7200, // 2 hours ago
		},
		{
			ID:      "new-container-456",
			Names:   []string{"/recac-agent-NEW"},
			Created: now - 1800, // 30 mins ago
		},
	}

	// Expectations
	mockDocker.On("ListContainers", ctx, mock.MatchedBy(func(opts container.ListOptions) bool {
		return opts.All == true && opts.Filters.ExactMatch("label", "created-by=recac-orchestrator")
	})).Return(containers, nil)

	mockDocker.On("RemoveContainer", ctx, "old-container-123", true).Return(nil)

	// Execute
	err := janitor.Cleanup(ctx)

	// Verify
	assert.NoError(t, err)
	mockDocker.AssertExpectations(t)
	mockDocker.AssertNotCalled(t, "RemoveContainer", ctx, "new-container-456", mock.Anything)
}

func TestJanitor_Cleanup_ListError(t *testing.T) {
	mockDocker := new(MockDockerClient)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	janitor := NewJanitor(mockDocker, 1*time.Hour, 10*time.Minute, logger)
	ctx := context.Background()

	mockDocker.On("ListContainers", ctx, mock.Anything).Return([]types.Container{}, assert.AnError)

	err := janitor.Cleanup(ctx)
	assert.Error(t, err)
}
