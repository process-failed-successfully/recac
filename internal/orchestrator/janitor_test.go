package orchestrator

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestJanitor_Cleanup(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	mockDocker := new(MockDockerClient)

	config := JanitorConfig{
		MaxAge:   1 * time.Hour,
		Interval: 1 * time.Minute,
		DryRun:   false,
	}

	janitor := NewJanitor(logger, mockDocker, config)

	now := time.Now()

	// Setup containers
	containers := []types.Container{
		{
			ID:      "old-container",
			Names:   []string{"/recac-agent-OLD"},
			Created: now.Add(-2 * time.Hour).Unix(), // Should be removed
		},
		{
			ID:      "new-container",
			Names:   []string{"/recac-agent-NEW"},
			Created: now.Add(-30 * time.Minute).Unix(), // Should be kept
		},
	}

	// Expectations
	mockDocker.On("ListContainers", mock.Anything, mock.MatchedBy(func(opts interface{}) bool {
		// Verify filters are set correctly?
		// Hard to verify filters.NewArgs internal structure without type assertion
		return true
	})).Return(containers, nil)

	mockDocker.On("RemoveContainer", mock.Anything, "old-container", true).Return(nil)

	// Run Cleanup
	err := janitor.Cleanup(context.Background())
	assert.NoError(t, err)

	mockDocker.AssertExpectations(t)
	mockDocker.AssertNotCalled(t, "RemoveContainer", mock.Anything, "new-container", mock.Anything)
}

func TestJanitor_Cleanup_DryRun(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	mockDocker := new(MockDockerClient)

	config := JanitorConfig{
		MaxAge:   1 * time.Hour,
		Interval: 1 * time.Minute,
		DryRun:   true,
	}

	janitor := NewJanitor(logger, mockDocker, config)

	now := time.Now()

	containers := []types.Container{
		{
			ID:      "old-container",
			Created: now.Add(-2 * time.Hour).Unix(),
		},
	}

	mockDocker.On("ListContainers", mock.Anything, mock.Anything).Return(containers, nil)

	// Should NOT call RemoveContainer

	err := janitor.Cleanup(context.Background())
	assert.NoError(t, err)

	mockDocker.AssertNotCalled(t, "RemoveContainer")
}
