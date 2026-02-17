package orchestrator

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/stretchr/testify/mock"
)

func TestJanitor_Cleanup(t *testing.T) {
	mockDocker := new(MockDockerClient)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	janitor := NewJanitor(mockDocker, logger, 1*time.Minute, 1*time.Hour, false)

	ctx := context.Background()

	// Mock ListContainers
	now := time.Now()
	oldTime := now.Add(-2 * time.Hour).Unix()
	newTime := now.Add(-30 * time.Minute).Unix()

	containers := []types.Container{
		{ID: "old-container", Created: oldTime, Names: []string{"/old"}},
		{ID: "new-container", Created: newTime, Names: []string{"/new"}},
	}

	mockDocker.On("ListContainers", ctx, mock.MatchedBy(func(opts container.ListOptions) bool {
		// Verify filters
		// filters.Args isn't easily inspectable via public API usually, but we can check if it's set
		return opts.All == true && opts.Filters.Len() > 0
	})).Return(containers, nil)

	// Mock RemoveContainer only for old container
	mockDocker.On("RemoveContainer", ctx, "old-container", true).Return(nil)

	janitor.cleanup(ctx)

	mockDocker.AssertExpectations(t)
	mockDocker.AssertNotCalled(t, "RemoveContainer", ctx, "new-container", true)
}

func TestJanitor_Cleanup_DryRun(t *testing.T) {
	mockDocker := new(MockDockerClient)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	janitor := NewJanitor(mockDocker, logger, 1*time.Minute, 1*time.Hour, true)

	ctx := context.Background()

	now := time.Now()
	oldTime := now.Add(-2 * time.Hour).Unix()

	containers := []types.Container{
		{ID: "old-container", Created: oldTime},
	}

	mockDocker.On("ListContainers", ctx, mock.Anything).Return(containers, nil)

	janitor.cleanup(ctx)

	mockDocker.AssertNotCalled(t, "RemoveContainer")
}
