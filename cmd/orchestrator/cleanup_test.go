package main

import (
	"context"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/stretchr/testify/mock"
)

type MockCleanupDockerClient struct {
	mock.Mock
}

func (m *MockCleanupDockerClient) CheckDaemon(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockCleanupDockerClient) ListContainers(ctx context.Context, options container.ListOptions) ([]types.Container, error) {
	args := m.Called(ctx, options)
	return args.Get(0).([]types.Container), args.Error(1)
}

func (m *MockCleanupDockerClient) RemoveContainer(ctx context.Context, containerID string, force bool) error {
	args := m.Called(ctx, containerID, force)
	return args.Error(0)
}

func (m *MockCleanupDockerClient) Close() error {
	return nil
}

func TestCleanupCmd(t *testing.T) {
	originalFactory := dockerClientFactory
	defer func() { dockerClientFactory = originalFactory }()

	mockClient := new(MockCleanupDockerClient)
	dockerClientFactory = func() (cleanupDockerClient, error) {
		return mockClient, nil
	}

	// Setup data
	now := time.Now()
	oldTime := now.Add(-48 * time.Hour).Unix()
	newTime := now.Add(-1 * time.Hour).Unix()

	containers := []types.Container{
		{ID: "old-container", Created: oldTime, Names: []string{"old"}, State: "exited"},
		{ID: "new-container", Created: newTime, Names: []string{"new"}, State: "running"},
	}

	// Expectations
	mockClient.On("CheckDaemon", mock.Anything).Return(nil)

	// Expect ListContainers with correct filter
	mockClient.On("ListContainers", mock.Anything, mock.MatchedBy(func(opts container.ListOptions) bool {
		return opts.All == true && opts.Filters.ExactMatch("label", "created-by=recac-orchestrator")
	})).Return(containers, nil)

	// Expect RemoveContainer only for old container
	mockClient.On("RemoveContainer", mock.Anything, "old-container", false).Return(nil)

	// Set flags directly on the command for this test run
	// Note: flags are package global variables in cleanup.go
	// But we should reset them
	cleanupOlderThan = 24 * time.Hour
	cleanupForce = false
	cleanupDryRun = false

	// Run logic directly
	err := cleanupCmd.RunE(cleanupCmd, []string{})
	if err != nil {
		t.Fatalf("RunE failed: %v", err)
	}

	mockClient.AssertExpectations(t)
	mockClient.AssertNotCalled(t, "RemoveContainer", mock.Anything, "new-container", mock.Anything)
}

func TestCleanupCmd_DryRun(t *testing.T) {
	originalFactory := dockerClientFactory
	defer func() { dockerClientFactory = originalFactory }()

	mockClient := new(MockCleanupDockerClient)
	dockerClientFactory = func() (cleanupDockerClient, error) {
		return mockClient, nil
	}

	// Setup data
	now := time.Now()
	oldTime := now.Add(-48 * time.Hour).Unix()

	containers := []types.Container{
		{ID: "old-container", Created: oldTime, Names: []string{"old"}, State: "exited"},
	}

	// Expectations
	mockClient.On("CheckDaemon", mock.Anything).Return(nil)
	mockClient.On("ListContainers", mock.Anything, mock.Anything).Return(containers, nil)

	// Set flags
	cleanupOlderThan = 24 * time.Hour
	cleanupDryRun = true

	// Run logic
	err := cleanupCmd.RunE(cleanupCmd, []string{})
	if err != nil {
		t.Fatalf("RunE failed: %v", err)
	}

	// Assert RemoveContainer was NOT called
	mockClient.AssertNotCalled(t, "RemoveContainer", mock.Anything, mock.Anything, mock.Anything)
}
