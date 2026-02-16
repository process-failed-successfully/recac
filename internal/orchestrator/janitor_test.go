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
	maxAge := 2 * time.Hour
	janitor := NewJanitor(mockDocker, logger, 1*time.Minute, maxAge)

	ctx := context.Background()

	// Mock ListContainers
	// We return 2 containers: one old (should be removed), one new (should be kept)
	oldContainer := types.Container{
		ID:      "old-container-id-long", // Ensure long enough for slicing check
		Created: time.Now().Add(-3 * time.Hour).Unix(),
		Names:   []string{"/old-agent"},
	}
	newContainer := types.Container{
		ID:      "new-container-id-long",
		Created: time.Now().Add(-1 * time.Hour).Unix(),
		Names:   []string{"/new-agent"},
	}

	mockDocker.On("ListContainers", ctx, mock.MatchedBy(func(opts container.ListOptions) bool {
		// Filter args check is complex as exact values depend on internal impl
		return opts.All == true && opts.Filters.ExactMatch("label", "created-by=recac-orchestrator")
	})).Return([]types.Container{oldContainer, newContainer}, nil).Once()

	// Mock RemoveContainer for old container
	mockDocker.On("RemoveContainer", ctx, oldContainer.ID, true).Return(nil).Once()

	// Ensure new container is NOT removed
	// Since we mock strictly, if RemoveContainer is called for new-container-id, test will fail (unexpected call)

	janitor.cleanup(ctx)

	mockDocker.AssertExpectations(t)
}

func TestJanitor_Cleanup_ListError(t *testing.T) {
	mockDocker := new(MockDockerClient)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	janitor := NewJanitor(mockDocker, logger, 1*time.Minute, 2*time.Hour)

	ctx := context.Background()

	// List fails
	mockDocker.On("ListContainers", ctx, mock.Anything).Return(nil, assert.AnError).Once()

	janitor.cleanup(ctx)
	// Should log error and return

	mockDocker.AssertExpectations(t)
}

func TestJanitor_Cleanup_RemoveError(t *testing.T) {
	mockDocker := new(MockDockerClient)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	janitor := NewJanitor(mockDocker, logger, 1*time.Minute, 2*time.Hour)

	ctx := context.Background()

	// List succeeds, Remove fails
	oldContainer := types.Container{
		ID:      "old-container-id-long",
		Created: time.Now().Add(-3 * time.Hour).Unix(),
	}
	mockDocker.On("ListContainers", ctx, mock.Anything).Return([]types.Container{oldContainer}, nil).Once()
	mockDocker.On("RemoveContainer", ctx, oldContainer.ID, true).Return(assert.AnError).Once()

	janitor.cleanup(ctx)
	// Should log error and continue

	mockDocker.AssertExpectations(t)
}
