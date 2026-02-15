package main

import (
	"context"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/stretchr/testify/assert"
)

// Mock implementation of CleanerDockerClient
type mockCleaner struct {
	listContainersFunc  func(ctx context.Context, options container.ListOptions) ([]types.Container, error)
	removeContainerFunc func(ctx context.Context, containerID string, force bool) error
	closeFunc           func() error
}

func (m *mockCleaner) ListContainers(ctx context.Context, options container.ListOptions) ([]types.Container, error) {
	if m.listContainersFunc != nil {
		return m.listContainersFunc(ctx, options)
	}
	return nil, nil
}

func (m *mockCleaner) RemoveContainer(ctx context.Context, containerID string, force bool) error {
	if m.removeContainerFunc != nil {
		return m.removeContainerFunc(ctx, containerID, force)
	}
	return nil
}

func (m *mockCleaner) Close() error {
	if m.closeFunc != nil {
		return m.closeFunc()
	}
	return nil
}

func TestCleanupCmd(t *testing.T) {
	// Backup and restore factory
	origFactory := cleanerDockerFactory
	defer func() { cleanerDockerFactory = origFactory }()

	t.Run("Dry Run - Does Not Remove", func(t *testing.T) {
		mockClient := &mockCleaner{}
		listCalled := false
		removeCalled := false

		mockClient.listContainersFunc = func(ctx context.Context, options container.ListOptions) ([]types.Container, error) {
			listCalled = true
			// Returns one stale container
			return []types.Container{
				{
					ID:      "stale-container",
					Names:   []string{"/recac-agent-123"},
					Created: time.Now().Add(-25 * time.Hour).Unix(),
					Labels:  map[string]string{"work-item": "123"},
				},
			}, nil
		}

		mockClient.removeContainerFunc = func(ctx context.Context, containerID string, force bool) error {
			removeCalled = true
			return nil
		}

		cleanerDockerFactory = func() (CleanerDockerClient, error) {
			return mockClient, nil
		}

		// Set flags
		cleanupCmd.Flags().Set("older-than", "24h")
		cleanupCmd.Flags().Set("dry-run", "true")

		cleanupCmd.Run(cleanupCmd, []string{})

		assert.True(t, listCalled)
		assert.False(t, removeCalled, "RemoveContainer should not be called in dry-run mode")
	})

	t.Run("Real Run - Removes Stale", func(t *testing.T) {
		mockClient := &mockCleaner{}
		listCalled := false
		removeCalled := false

		mockClient.listContainersFunc = func(ctx context.Context, options container.ListOptions) ([]types.Container, error) {
			listCalled = true
			return []types.Container{
				{
					ID:      "stale-container",
					Names:   []string{"/recac-agent-123"},
					Created: time.Now().Add(-25 * time.Hour).Unix(),
					Labels:  map[string]string{"work-item": "123"},
				},
				{
					ID:      "fresh-container",
					Names:   []string{"/recac-agent-456"},
					Created: time.Now().Add(-1 * time.Hour).Unix(),
					Labels:  map[string]string{"work-item": "456"},
				},
			}, nil
		}

		mockClient.removeContainerFunc = func(ctx context.Context, containerID string, force bool) error {
			if containerID == "stale-container" {
				removeCalled = true
			} else {
				t.Errorf("Attempted to remove non-stale container %s", containerID)
			}
			return nil
		}

		cleanerDockerFactory = func() (CleanerDockerClient, error) {
			return mockClient, nil
		}

		// Set flags
		cleanupCmd.Flags().Set("older-than", "24h")
		cleanupCmd.Flags().Set("dry-run", "false")

		cleanupCmd.Run(cleanupCmd, []string{})

		assert.True(t, listCalled)
		assert.True(t, removeCalled, "RemoveContainer should be called for stale container")
	})
}
