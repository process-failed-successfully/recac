package main

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Mock implementation of SandboxDockerClient
type mockSandboxClient struct {
	runContainerFunc    func(ctx context.Context, imageRef string, workspace string, extraBinds []string, ports []string, user string) (string, error)
	execInteractiveFunc func(ctx context.Context, containerID string, cmd []string) error
	stopContainerFunc   func(ctx context.Context, containerID string) error
	removeContainerFunc func(ctx context.Context, containerID string, force bool) error
	closeFunc           func() error
}

func (m *mockSandboxClient) RunContainer(ctx context.Context, imageRef string, workspace string, extraBinds []string, ports []string, user string) (string, error) {
	if m.runContainerFunc != nil {
		return m.runContainerFunc(ctx, imageRef, workspace, extraBinds, ports, user)
	}
	return "mock-container-id", nil
}

func (m *mockSandboxClient) ExecInteractive(ctx context.Context, containerID string, cmd []string) error {
	if m.execInteractiveFunc != nil {
		return m.execInteractiveFunc(ctx, containerID, cmd)
	}
	return nil
}

func (m *mockSandboxClient) StopContainer(ctx context.Context, containerID string) error {
	if m.stopContainerFunc != nil {
		return m.stopContainerFunc(ctx, containerID)
	}
	return nil
}

func (m *mockSandboxClient) RemoveContainer(ctx context.Context, containerID string, force bool) error {
	if m.removeContainerFunc != nil {
		return m.removeContainerFunc(ctx, containerID, force)
	}
	return nil
}

func (m *mockSandboxClient) Close() error {
	if m.closeFunc != nil {
		return m.closeFunc()
	}
	return nil
}

func TestSandboxCmd(t *testing.T) {
	// Save original factory and restore after test
	origFactory := sandboxDockerFactory
	defer func() { sandboxDockerFactory = origFactory }()

	t.Run("Successful Run", func(t *testing.T) {
		mockClient := &mockSandboxClient{}
		runCalled := false
		execCalled := false
		stopCalled := false
		removeCalled := false

		mockClient.runContainerFunc = func(ctx context.Context, imageRef string, workspace string, extraBinds []string, ports []string, user string) (string, error) {
			runCalled = true
			assert.Equal(t, "ghcr.io/process-failed-successfully/recac-agent:latest", imageRef, "Should use default image")
			assert.NotEmpty(t, workspace, "Workspace should not be empty")
			return "test-container-id", nil
		}

		mockClient.execInteractiveFunc = func(ctx context.Context, containerID string, cmd []string) error {
			execCalled = true
			assert.Equal(t, "test-container-id", containerID)
			return nil
		}

		mockClient.stopContainerFunc = func(ctx context.Context, containerID string) error {
			stopCalled = true
			assert.Equal(t, "test-container-id", containerID)
			return nil
		}

		mockClient.removeContainerFunc = func(ctx context.Context, containerID string, force bool) error {
			removeCalled = true
			assert.Equal(t, "test-container-id", containerID)
			return nil
		}

		sandboxDockerFactory = func(project string) (SandboxDockerClient, error) {
			return mockClient, nil
		}

		// Execute
		err := sandboxCmd.RunE(sandboxCmd, []string{})
		assert.NoError(t, err)

		assert.True(t, runCalled, "RunContainer should be called")
		assert.True(t, execCalled, "ExecInteractive should be called")
		assert.True(t, stopCalled, "StopContainer should be called")
		assert.True(t, removeCalled, "RemoveContainer should be called")
	})

	t.Run("Custom Image and User", func(t *testing.T) {
		mockClient := &mockSandboxClient{}
		runCalled := false

		mockClient.runContainerFunc = func(ctx context.Context, imageRef string, workspace string, extraBinds []string, ports []string, user string) (string, error) {
			runCalled = true
			assert.Equal(t, "custom-image", imageRef)
			assert.Equal(t, "custom-user", user)
			return "test-container-id", nil
		}

		sandboxDockerFactory = func(project string) (SandboxDockerClient, error) {
			return mockClient, nil
		}

		// Set flags
		sandboxCmd.Flags().Set("image", "custom-image")
		sandboxCmd.Flags().Set("user", "custom-user")
		defer func() {
			sandboxCmd.Flags().Set("image", "")
			sandboxCmd.Flags().Set("user", "")
		}()

		err := sandboxCmd.RunE(sandboxCmd, []string{})
		assert.NoError(t, err)
		assert.True(t, runCalled)
	})

	t.Run("Exec Failure Fallback", func(t *testing.T) {
		mockClient := &mockSandboxClient{}
		execCalls := 0

		mockClient.execInteractiveFunc = func(ctx context.Context, containerID string, cmd []string) error {
			execCalls++
			if execCalls == 1 {
				return fmt.Errorf("bash not found")
			}
			return nil
		}

		sandboxDockerFactory = func(project string) (SandboxDockerClient, error) {
			return mockClient, nil
		}

		err := sandboxCmd.RunE(sandboxCmd, []string{})
		assert.NoError(t, err)
		assert.Equal(t, 2, execCalls, "Should attempt fallback to sh")
	})
}
