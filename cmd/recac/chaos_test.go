package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/stretchr/testify/assert"
)

type MockChaosDockerClient struct {
	ListContainersFunc func(ctx context.Context, options container.ListOptions) ([]types.Container, error)
	KillContainerFunc  func(ctx context.Context, containerID, signal string) error
	CloseFunc          func() error
}

func (m *MockChaosDockerClient) ListContainers(ctx context.Context, options container.ListOptions) ([]types.Container, error) {
	if m.ListContainersFunc != nil {
		return m.ListContainersFunc(ctx, options)
	}
	return nil, nil
}

func (m *MockChaosDockerClient) KillContainer(ctx context.Context, containerID, signal string) error {
	if m.KillContainerFunc != nil {
		return m.KillContainerFunc(ctx, containerID, signal)
	}
	return nil
}

func (m *MockChaosDockerClient) Close() error {
	if m.CloseFunc != nil {
		return m.CloseFunc()
	}
	return nil
}

func TestChaosDocker(t *testing.T) {
	// Backup and restore factory
	originalFactory := chaosDockerClientFactory
	defer func() { chaosDockerClientFactory = originalFactory }()

	t.Run("Kill matching containers", func(t *testing.T) {
		killed := make(map[string]bool)
		mock := &MockChaosDockerClient{
			ListContainersFunc: func(ctx context.Context, options container.ListOptions) ([]types.Container, error) {
				return []types.Container{
					{ID: "id1", Names: []string{"/test-app-1"}},
					{ID: "id2", Names: []string{"/other-app"}},
					{ID: "id3", Names: []string{"/test-app-2"}},
				}, nil
			},
			KillContainerFunc: func(ctx context.Context, containerID, signal string) error {
				killed[containerID] = true
				return nil
			},
		}
		chaosDockerClientFactory = func(project string) (chaosDockerClient, error) {
			return mock, nil
		}

		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()
		rootCmd.SetContext(ctx)

		// Run command
		// We use executeCommand but expected it to return when context expires
		_, err := executeCommand(rootCmd, "chaos", "docker", "--pattern", "test-app-*", "--interval", "10ms")
		assert.NoError(t, err)

		// We expect at least one kill attempt if the interval is short enough
		// With 10ms interval and 50ms duration, it should run ~5 times.
		// Since ListContainers returns same list, it might try to kill same container again.
		// chaos count default is 1.

		assert.True(t, killed["id1"] || killed["id3"], "Should have killed matching containers")
		assert.False(t, killed["id2"], "Should not kill non-matching containers")
	})

	t.Run("Dry Run", func(t *testing.T) {
		killed := make(map[string]bool)
		mock := &MockChaosDockerClient{
			ListContainersFunc: func(ctx context.Context, options container.ListOptions) ([]types.Container, error) {
				return []types.Container{
					{ID: "id1", Names: []string{"/test-app-1"}},
				}, nil
			},
			KillContainerFunc: func(ctx context.Context, containerID, signal string) error {
				killed[containerID] = true
				return nil
			},
		}
		chaosDockerClientFactory = func(project string) (chaosDockerClient, error) {
			return mock, nil
		}

		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		defer cancel()
		rootCmd.SetContext(ctx)

		output, err := executeCommand(rootCmd, "chaos", "docker", "--pattern", "test-app-*", "--interval", "10ms", "--dry-run")
		assert.NoError(t, err)

		assert.Empty(t, killed, "Should not kill any containers in dry run")
		assert.Contains(t, output, "[DRY RUN] Would kill")
	})
}

func TestChaosFile(t *testing.T) {
	// Setup temp dir
	tmpDir, err := os.MkdirTemp("", "chaos-test")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create a file
	filePath := filepath.Join(tmpDir, "test.txt")
	err = os.WriteFile(filePath, []byte("original"), 0644)
	assert.NoError(t, err)

	// Run chaos file
	// We use rate 1.1 to ensure modification
	_, err = executeCommand(rootCmd, "chaos", "file", "--path", tmpDir, "--rate", "1.1")
	assert.NoError(t, err)

	// Verify modification
	content, err := os.ReadFile(filePath)
	assert.NoError(t, err)
	assert.Contains(t, string(content), "# CHAOS WAS HERE")
}

func TestChaosFile_DryRun(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "chaos-test")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	filePath := filepath.Join(tmpDir, "test.txt")
	err = os.WriteFile(filePath, []byte("original"), 0644)
	assert.NoError(t, err)

	output, err := executeCommand(rootCmd, "chaos", "file", "--path", tmpDir, "--rate", "1.1", "--dry-run")
	assert.NoError(t, err)
	assert.Contains(t, output, "[DRY RUN] Would corrupt")

	content, err := os.ReadFile(filePath)
	assert.NoError(t, err)
	assert.Equal(t, "original", string(content))
}

func TestChaosStress(t *testing.T) {
	// Backup globals
	oldCPU := chaosCPU
	oldMem := chaosMemory
	oldDur := chaosDuration
	defer func() {
		chaosCPU = oldCPU
		chaosMemory = oldMem
		chaosDuration = oldDur
	}()

	chaosCPU = 1
	chaosMemory = 1
	chaosDuration = 10 * time.Millisecond

	cmd := &cobra.Command{}
	err := runChaosStress(cmd, nil)
	assert.NoError(t, err)
}

func TestChaosDocker_Errors(t *testing.T) {
	originalFactory := chaosDockerClientFactory
	defer func() { chaosDockerClientFactory = originalFactory }()

	// Backup globals
	oldPattern := chaosPattern
	oldCount := chaosCount
	oldDryRun := chaosDryRun
	defer func() {
		chaosPattern = oldPattern
		chaosCount = oldCount
		chaosDryRun = oldDryRun
	}()

	// Reset flags
	chaosDryRun = false

	t.Run("Factory Error", func(t *testing.T) {
		chaosDockerClientFactory = func(project string) (chaosDockerClient, error) {
			return nil, errors.New("factory failed")
		}
		chaosPattern = "anything"
		err := runChaosDocker(&cobra.Command{}, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "factory failed")
	})

	t.Run("List Error", func(t *testing.T) {
		mock := &MockChaosDockerClient{
			ListContainersFunc: func(ctx context.Context, options container.ListOptions) ([]types.Container, error) {
				return nil, errors.New("list failed")
			},
		}
		err := killRandomContainers(context.Background(), &cobra.Command{}, mock)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "list failed")
	})

	t.Run("Kill Error", func(t *testing.T) {
		mock := &MockChaosDockerClient{
			ListContainersFunc: func(ctx context.Context, options container.ListOptions) ([]types.Container, error) {
				return []types.Container{{ID: "id1", Names: []string{"/target"}}}, nil
			},
			KillContainerFunc: func(ctx context.Context, containerID, signal string) error {
				return errors.New("kill failed")
			},
		}
		chaosPattern = "target"
		chaosCount = 1
		err := killRandomContainers(context.Background(), &cobra.Command{}, mock)
		assert.Error(t, err)
		if err != nil {
			assert.Contains(t, err.Error(), "kill failed")
		}
	})

	t.Run("No Matches", func(t *testing.T) {
		mock := &MockChaosDockerClient{
			ListContainersFunc: func(ctx context.Context, options container.ListOptions) ([]types.Container, error) {
				return []types.Container{{ID: "id1", Names: []string{"/other"}}}, nil
			},
		}
		chaosPattern = "target"
		err := killRandomContainers(context.Background(), &cobra.Command{}, mock)
		assert.NoError(t, err)
	})
}

func TestChaosFile_Errors(t *testing.T) {
	// Empty path
	chaosPath = ""
	err := runChaosFile(&cobra.Command{}, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "path is required")

	// Walk error
	chaosPath = "/non/existent"
	err = runChaosFile(&cobra.Command{}, nil)
	assert.Error(t, err)
}
