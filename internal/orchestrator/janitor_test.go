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
	janitor := NewJanitor(mockDocker, logger, 24*time.Hour, false)

	ctx := context.Background()
	now := time.Now()

	containers := []types.Container{
		{
			ID:      "old-container",
			Created: now.Add(-48 * time.Hour).Unix(),
			Names:   []string{"/recac-agent-old"},
		},
		{
			ID:      "new-container",
			Created: now.Add(-1 * time.Hour).Unix(),
			Names:   []string{"/recac-agent-new"},
		},
	}

	// ListContainers returns mixed containers, filter is applied in call
	mockDocker.On("ListContainers", ctx, mock.MatchedBy(func(opts container.ListOptions) bool {
		// Verify filter
		return opts.All == true && opts.Filters.ExactMatch("label", "created-by=recac-orchestrator")
	})).Return(containers, nil)

	// RemoveContainer should be called for old-container
	mockDocker.On("RemoveContainer", ctx, "old-container", true).Return(nil)

	err := janitor.Cleanup(ctx)
	assert.NoError(t, err)

	mockDocker.AssertCalled(t, "RemoveContainer", ctx, "old-container", true)
	mockDocker.AssertNotCalled(t, "RemoveContainer", ctx, "new-container", mock.Anything)
}

func TestJanitor_Cleanup_DryRun(t *testing.T) {
	mockDocker := new(MockDockerClient)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	janitor := NewJanitor(mockDocker, logger, 24*time.Hour, true)

	ctx := context.Background()
	now := time.Now()

	containers := []types.Container{
		{
			ID:      "old-container",
			Created: now.Add(-48 * time.Hour).Unix(),
			Names:   []string{"/recac-agent-old"},
		},
	}

	mockDocker.On("ListContainers", ctx, mock.Anything).Return(containers, nil)

	err := janitor.Cleanup(ctx)
	assert.NoError(t, err)

	mockDocker.AssertNotCalled(t, "RemoveContainer", ctx, "old-container", mock.Anything)
}
