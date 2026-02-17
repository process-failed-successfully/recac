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

// MockJanitorDockerClient is a mock implementation of JanitorDockerClient
type MockJanitorDockerClient struct {
	mock.Mock
}

func (m *MockJanitorDockerClient) ListContainers(ctx context.Context, options container.ListOptions) ([]types.Container, error) {
	args := m.Called(ctx, options)
	return args.Get(0).([]types.Container), args.Error(1)
}

func (m *MockJanitorDockerClient) RemoveContainer(ctx context.Context, containerID string, force bool) error {
	args := m.Called(ctx, containerID, force)
	return args.Error(0)
}

func TestJanitor_Cleanup_Exited(t *testing.T) {
	mockClient := new(MockJanitorDockerClient)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	janitor := NewJanitor(mockClient, "test-project", 1*time.Hour, true)

	// Setup mock to return an exited container
	mockClient.On("ListContainers", mock.Anything, mock.MatchedBy(func(opts container.ListOptions) bool {
		return opts.All == true
	})).Return([]types.Container{
		{ID: "container12345", State: "exited", Created: time.Now().Unix()},
	}, nil)

	mockClient.On("RemoveContainer", mock.Anything, "container12345", true).Return(nil)

	janitor.cleanup(context.Background(), logger)

	mockClient.AssertExpectations(t)
}

func TestJanitor_Cleanup_OldRunning(t *testing.T) {
	mockClient := new(MockJanitorDockerClient)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	janitor := NewJanitor(mockClient, "test-project", 1*time.Hour, true)

	// Setup mock to return an old running container
	oldTime := time.Now().Add(-2 * time.Hour).Unix()
	mockClient.On("ListContainers", mock.Anything, mock.Anything).Return([]types.Container{
		{ID: "container12345", State: "running", Created: oldTime},
	}, nil)

	mockClient.On("RemoveContainer", mock.Anything, "container12345", true).Return(nil)

	janitor.cleanup(context.Background(), logger)

	mockClient.AssertExpectations(t)
}

func TestJanitor_NoCleanup_YoungRunning(t *testing.T) {
	mockClient := new(MockJanitorDockerClient)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	janitor := NewJanitor(mockClient, "test-project", 1*time.Hour, true)

	// Setup mock to return a young running container
	youngTime := time.Now().Add(-30 * time.Minute).Unix()
	mockClient.On("ListContainers", mock.Anything, mock.Anything).Return([]types.Container{
		{ID: "container12345", State: "running", Created: youngTime},
	}, nil)

	// RemoveContainer should NOT be called

	janitor.cleanup(context.Background(), logger)

	mockClient.AssertExpectations(t)
}
