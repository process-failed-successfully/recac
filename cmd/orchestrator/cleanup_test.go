package main

import (
	"context"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockCleanerDockerClient
type MockCleanerDockerClient struct {
	mock.Mock
}

func (m *MockCleanerDockerClient) RunContainer(ctx context.Context, image, workspace string, binds, env []string, labels map[string]string, user string) (string, error) {
	args := m.Called(ctx, image, workspace, binds, env, labels, user)
	return args.String(0), args.Error(1)
}

func (m *MockCleanerDockerClient) StopContainer(ctx context.Context, containerID string) error {
	args := m.Called(ctx, containerID)
	return args.Error(0)
}

func (m *MockCleanerDockerClient) Exec(ctx context.Context, containerID string, cmd []string) (string, error) {
	args := m.Called(ctx, containerID, cmd)
	return args.String(0), args.Error(1)
}

func (m *MockCleanerDockerClient) RemoveContainer(ctx context.Context, containerID string, force bool) error {
	args := m.Called(ctx, containerID, force)
	return args.Error(0)
}

func (m *MockCleanerDockerClient) ListContainers(ctx context.Context, options container.ListOptions) ([]types.Container, error) {
	args := m.Called(ctx, options)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]types.Container), args.Error(1)
}

func (m *MockCleanerDockerClient) Close() error {
	args := m.Called()
	return args.Error(0)
}

func TestCleanupCmd(t *testing.T) {
	// Mock Factory
	mockClient := new(MockCleanerDockerClient)
	origFactory := dockerClientFactory
	dockerClientFactory = func(project string) (CleanerDockerClient, error) {
		return mockClient, nil
	}
	defer func() { dockerClientFactory = origFactory }()

	// Test Cases
	t.Run("No Containers", func(t *testing.T) {
		mockClient.On("ListContainers", mock.Anything, mock.Anything).Return([]types.Container{}, nil).Once()
		mockClient.On("Close").Return(nil).Once()

		rootCmd.SetArgs([]string{"cleanup", "--older-than", "1h"})
		err := rootCmd.Execute()
		assert.NoError(t, err)
		mockClient.AssertExpectations(t)
	})

	t.Run("Remove Old Containers", func(t *testing.T) {
		oldTime := time.Now().Add(-2 * time.Hour).Unix()
		newTime := time.Now().Unix()

		containers := []types.Container{
			{ID: "old-1-longer-id", Names: []string{"/old-1"}, Created: oldTime, State: "exited"},
			{ID: "new-1-longer-id", Names: []string{"/new-1"}, Created: newTime, State: "running"},
			{ID: "old-running-id", Names: []string{"/old-running"}, Created: oldTime, State: "running"},
		}

		mockClient.On("ListContainers", mock.Anything, mock.Anything).Return(containers, nil).Once()
		mockClient.On("RemoveContainer", mock.Anything, "old-1-longer-id", false).Return(nil).Once()
		mockClient.On("StopContainer", mock.Anything, "old-running-id").Return(nil).Once()
		mockClient.On("RemoveContainer", mock.Anything, "old-running-id", false).Return(nil).Once()
		mockClient.On("Close").Return(nil).Once()

		rootCmd.SetArgs([]string{"cleanup", "--older-than", "1h"})
		err := rootCmd.Execute()
		assert.NoError(t, err)
		mockClient.AssertExpectations(t)
	})

	t.Run("Force Cleanup", func(t *testing.T) {
		oldTime := time.Now().Add(-2 * time.Hour).Unix()
		containers := []types.Container{
			{ID: "old-running-id", Names: []string{"/old-running"}, Created: oldTime, State: "running"},
		}

		mockClient.On("ListContainers", mock.Anything, mock.Anything).Return(containers, nil).Once()
		// Stop might happen, but Remove should use force=true
		mockClient.On("StopContainer", mock.Anything, "old-running-id").Return(nil).Once()
		mockClient.On("RemoveContainer", mock.Anything, "old-running-id", true).Return(nil).Once()
		mockClient.On("Close").Return(nil).Once()

		rootCmd.SetArgs([]string{"cleanup", "--older-than", "1h", "--force"})
		err := rootCmd.Execute()
		assert.NoError(t, err)
		mockClient.AssertExpectations(t)
	})
}
