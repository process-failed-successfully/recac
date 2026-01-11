package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"recac/internal/docker"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockDockerClient is a field in the test struct to control Docker-related checks
var mockDockerClient *docker.MockAPI

// newTestDockerClient is a factory function that returns the mock client for tests
func newTestDockerClient(component string) (docker.APIClient, error) {
	if mockDockerClient == nil {
		return nil, errors.New("mock docker client not initialized")
	}
	return mockDockerClient, nil
}

func setupCheckTest(t *testing.T) {
	// Use a temporary file for config
	tmpfile, err := os.CreateTemp(t.TempDir(), "config.yaml")
	require.NoError(t, err)
	viper.SetConfigFile(tmpfile.Name())

	// Reset mock docker client for each test
	mockDockerClient = &docker.MockAPI{
		Images: make(map[string]bool),
	}

	// Override the docker client factory
	dockerNewClient = newTestDockerClient

	// Keep track of original PATH
	originalPath := os.Getenv("PATH")
	t.Cleanup(func() {
		dockerNewClient = docker.NewClient // Restore original factory
		os.Setenv("PATH", originalPath)
		viper.Reset()
	})
}

func TestCheckCommand(t *testing.T) {
	t.Run("all checks pass", func(t *testing.T) {
		setupCheckTest(t)
		// --- Arrange ---
		// Create a dummy config file
		viper.Set("agent_provider", "test")
		require.NoError(t, viper.WriteConfig())
		// Set up mock docker client for success
		mockDockerClient.DaemonReachable = true
		mockDockerClient.SocketAccessible = true
		mockDockerClient.Images["ubuntu:latest"] = true
		// Ensure git and go are on the path (default test environment)
		// --- Act ---
		output, err := executeCommand(rootCmd, "check")
		// --- Assert ---
		require.NoError(t, err)
		assert.Contains(t, output, "All checks passed!")
		assert.Contains(t, output, "✅ Config found")
		assert.Contains(t, output, "✅ Git installed")
		assert.Contains(t, output, "✅ Go installed")
		assert.Contains(t, output, "✅ Docker is available and ready")
	})

	t.Run("git not found", func(t *testing.T) {
		setupCheckTest(t)
		// --- Arrange ---
		viper.Set("agent_provider", "test")
		require.NoError(t, viper.WriteConfig())
		mockDockerClient.DaemonReachable = true
		mockDockerClient.SocketAccessible = true
		mockDockerClient.Images["ubuntu:latest"] = true
		// Manipulate PATH to hide git
		os.Setenv("PATH", "/tmp")
		// --- Act ---
		output, err := executeCommand(rootCmd, "check")
		// --- Assert ---
		assert.Error(t, err) // command should exit with 1
		assert.Contains(t, output, "Some checks failed")
		assert.Contains(t, output, "❌ Git: git binary not found in PATH")
	})

	t.Run("docker daemon unreachable", func(t *testing.T) {
		setupCheckTest(t)
		// --- Arrange ---
		viper.Set("agent_provider", "test")
		require.NoError(t, viper.WriteConfig())
		mockDockerClient.DaemonReachable = false // Simulate failure
		mockDockerClient.SocketAccessible = true
		mockDockerClient.Images["ubuntu:latest"] = true
		// --- Act ---
		output, err := executeCommand(rootCmd, "check")
		// --- Assert ---
		assert.Error(t, err)
		assert.Contains(t, output, "Some checks failed")
		assert.Contains(t, output, "❌ Docker: docker setup is incomplete")
		assert.Contains(t, output, "❌ Docker daemon is not reachable")
	})

	t.Run("missing docker image with fix", func(t *testing.T) {
		setupCheckTest(t)
		// --- Arrange ---
		viper.Set("agent_provider", "test")
		require.NoError(t, viper.WriteConfig())
		mockDockerClient.DaemonReachable = true
		mockDockerClient.SocketAccessible = true
		mockDockerClient.Images["ubuntu:latest"] = false // Image is missing
		// --- Act ---
		output, err := executeCommand(rootCmd, "check", "--fix")
		// --- Assert ---
		// The overall check should now pass because the fix was successful
		require.NoError(t, err)
		assert.Contains(t, output, "All checks passed!")
		assert.Contains(t, output, "🔧 Attempting to pull missing images...")
		assert.Contains(t, output, "✅ Auto-fix successfully pulled ubuntu:latest")
		// Verify the mock was called
		assert.Equal(t, 1, mockDockerClient.PullImageCalled, "PullImage should have been called once")
	})

	t.Run("missing config with fix", func(t *testing.T) {
		setupCheckTest(t)
		// --- Arrange ---
		// Don't create a config file
		// --- Act ---
		output, err := executeCommand(rootCmd, "check", "--fix")
		// --- Assert ---
		// This will still fail on other checks, but we want to see the config fix attempt
		assert.Contains(t, output, "🔧 Attempting to fix missing config file...")
		assert.Contains(t, output, "✅ Config fixed (created default)")
		// Verify the file was created
		_, statErr := os.Stat(viper.ConfigFileUsed())
		assert.NoError(t, statErr)
	})
}

// A simple mock for the Docker client to be used in tests.
// This is a simplified version, the real one is in internal/docker/mock_client.go
type MockDockerAPI struct {
	DaemonReachable  bool
	SocketAccessible bool
	Images           map[string]bool
	PullImageCalled  int
}

func (m *MockDockerAPI) CheckDaemon(ctx context.Context) error {
	if m.DaemonReachable {
		return nil
	}
	return errors.New("daemon not reachable")
}

func (m *MockDockerAPI) CheckSocket(ctx context.Context) error {
	if m.SocketAccessible {
		return nil
	}
	return errors.New("socket not accessible")
}

func (m *MockDockerAPI) CheckImage(ctx context.Context, ref string) (bool, error) {
	exists, found := m.Images[ref]
	if !found {
		return false, fmt.Errorf("image %s not in mock map", ref)
	}
	return exists, nil
}

func (m *MockDockerAPI) PullImage(ctx context.Context, ref string) error {
	m.PullImageCalled++
	m.Images[ref] = true // Simulate successful pull
	return nil
}

func (m *MockDockerAPI) Close() error { return nil }
func (m *MockDockerAPI) GetContainerLogs(ctx context.Context, containerID string, since string, follow bool) (string, error) {
	return "", nil
}
func (m *MockDockerAPI) GetImage(ctx context.Context, imageName string) (string, error) { return "", nil }
func (m *MockDockerAPI) Exec(ctx context.Context, containerID string, cmd []string) (int, string, error) {
	return 0, "", nil
}
func (m *MockDockerAPI) GetContainerMounts(ctx context.Context, containerID string) ([]string, error) {
	return nil, nil
}
func (m *MockDockerAPI) CreateContainer(ctx context.Context, imageName string, cmd []string, workspace string, mounts map[string]string) (string, error) {
	return "", nil
}
func (m *MockDockerAPI) StartContainer(ctx context.Context, containerID string) error { return nil }
func (m *MockDockerAPI) StopContainer(ctx context.Context, containerID string) error { return nil }
func (m *MockDockerAPI) RemoveContainer(ctx context.Context, containerID string) error { return nil }
func (m *MockDockerAPI) ListContainers(ctx context.Context, all bool) ([]string, error) {
	return nil, nil
}
func (m *MockDockerAPI) IsImageInstalled(ctx context.Context, imageName string) (bool, error) {
	return false, nil
}
