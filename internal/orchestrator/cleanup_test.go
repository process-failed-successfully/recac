package orchestrator

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type MockCleanerDockerClient struct {
	mock.Mock
}

func (m *MockCleanerDockerClient) ListContainers(ctx context.Context, options container.ListOptions) ([]types.Container, error) {
	args := m.Called(ctx, options)
	return args.Get(0).([]types.Container), args.Error(1)
}

func (m *MockCleanerDockerClient) RemoveContainer(ctx context.Context, containerID string, force bool) error {
	args := m.Called(ctx, containerID, force)
	return args.Error(0)
}

func TestCleanupContainers(t *testing.T) {
	// Discard logs
	logger := slog.New(slog.NewTextHandler(nil, &slog.HandlerOptions{Level: slog.LevelError}))
	ctx := context.Background()

	t.Run("DryRun", func(t *testing.T) {
		client := new(MockCleanerDockerClient)

		now := time.Now()
		oldContainer := types.Container{
			ID:      "old-container",
			Created: now.Add(-48 * time.Hour).Unix(),
			Names:   []string{"/old-container"},
		}
		newContainer := types.Container{
			ID:      "new-container",
			Created: now.Add(-1 * time.Hour).Unix(),
			Names:   []string{"/new-container"},
		}

		client.On("ListContainers", ctx, mock.Anything).Return([]types.Container{oldContainer, newContainer}, nil)

		// RemoveContainer should NOT be called because it's dry-run

		err := CleanupContainers(ctx, client, 24*time.Hour, true, logger)
		require.NoError(t, err)

		client.AssertExpectations(t)
	})

	t.Run("RealRun", func(t *testing.T) {
		client := new(MockCleanerDockerClient)

		now := time.Now()
		oldContainer := types.Container{
			ID:      "old-container",
			Created: now.Add(-48 * time.Hour).Unix(),
			Names:   []string{"/old-container"},
		}
		newContainer := types.Container{
			ID:      "new-container",
			Created: now.Add(-1 * time.Hour).Unix(),
			Names:   []string{"/new-container"},
		}

		client.On("ListContainers", ctx, mock.Anything).Return([]types.Container{oldContainer, newContainer}, nil)
		client.On("RemoveContainer", ctx, "old-container", true).Return(nil)

		err := CleanupContainers(ctx, client, 24*time.Hour, false, logger)
		require.NoError(t, err)

		client.AssertExpectations(t)
	})

	t.Run("ListError", func(t *testing.T) {
		client := new(MockCleanerDockerClient)

		client.On("ListContainers", ctx, mock.Anything).Return([]types.Container{}, assertError(t, "list error"))

		err := CleanupContainers(ctx, client, 24*time.Hour, false, logger)
		require.Error(t, err)
		require.Contains(t, err.Error(), "list error")

		client.AssertExpectations(t)
	})
}

// Helper to return a generic error without importing errors package unnecessarily
func assertError(t *testing.T, msg string) error {
    return &testError{msg}
}

type testError struct {
    msg string
}

func (e *testError) Error() string {
    return e.msg
}
