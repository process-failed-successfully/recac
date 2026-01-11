package main

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockDockerClient is a mock implementation of the dockerChecker interface for testing.
type mockDockerClient struct {
	checkDaemonFunc func(ctx context.Context) error
	checkSocketFunc func(ctx context.Context) error
	checkImageFunc  func(ctx context.Context, ref string) (bool, error)
	pullImageFunc   func(ctx context.Context, ref string) error
	closeFunc       func() error
}

func (m *mockDockerClient) CheckDaemon(ctx context.Context) error {
	if m.checkDaemonFunc != nil {
		return m.checkDaemonFunc(ctx)
	}
	return nil
}

func (m *mockDockerClient) CheckSocket(ctx context.Context) error {
	if m.checkSocketFunc != nil {
		return m.checkSocketFunc(ctx)
	}
	return nil
}

func (m *mockDockerClient) CheckImage(ctx context.Context, ref string) (bool, error) {
	if m.checkImageFunc != nil {
		return m.checkImageFunc(ctx, ref)
	}
	return true, nil
}

func (m *mockDockerClient) PullImage(ctx context.Context, ref string) error {
	if m.pullImageFunc != nil {
		return m.pullImageFunc(ctx, ref)
	}
	return nil
}

func (m *mockDockerClient) Close() error {
	if m.closeFunc != nil {
		return m.closeFunc()
	}
	return nil
}

// setupCheckTest creates a mock client and a temporary config file for isolated testing.
// It returns the mock client and the path to the temporary config file.
func setupCheckTest(t *testing.T) (*mockDockerClient, string) {
	t.Helper()

	// Create a temp file with a proper .yaml extension
	tmpfile, err := os.CreateTemp(t.TempDir(), "config-*.yaml")
	require.NoError(t, err)
	configPath := tmpfile.Name()
	require.NoError(t, tmpfile.Close())

	mockClient := &mockDockerClient{}

	// Override the newDockerClient factory to return our mock for the duration of the test
	originalNewDockerClient := newDockerClient
	newDockerClient = func(component string) (dockerChecker, error) {
		return mockClient, nil
	}
	t.Cleanup(func() { newDockerClient = originalNewDockerClient })

	// Keep track of and restore original PATH
	originalPath := os.Getenv("PATH")
	t.Cleanup(func() { os.Setenv("PATH", originalPath) })

	// Reset viper after each test to ensure no state leaks
	t.Cleanup(viper.Reset)

	return mockClient, configPath
}

func TestCheckCommand(t *testing.T) {
	t.Run("all checks pass", func(t *testing.T) {
		mockClient, configPath := setupCheckTest(t)
		// --- Arrange ---
		require.NoError(t, os.WriteFile(configPath, []byte("agent_provider: test"), 0600))
		mockClient.checkDaemonFunc = func(ctx context.Context) error { return nil }
		mockClient.checkSocketFunc = func(ctx context.Context) error { return nil }
		mockClient.checkImageFunc = func(ctx context.Context, ref string) (bool, error) { return true, nil }

		// --- Act ---
		output, err := executeCommand(rootCmd, "check", "--config", configPath)

		// --- Assert ---
		require.NoError(t, err)
		assert.Contains(t, output, "All checks passed!")
		assert.Contains(t, output, "✅ Config found")
		assert.Contains(t, output, "✅ Git installed")
		assert.Contains(t, output, "✅ Go installed")
		assert.Contains(t, output, "✅ Docker is available and ready")
	})

	t.Run("git not found", func(t *testing.T) {
		_, configPath := setupCheckTest(t)
		// --- Arrange ---
		require.NoError(t, os.WriteFile(configPath, []byte("agent_provider: test"), 0600))
		os.Setenv("PATH", "/tmp") // Manipulate PATH to hide git

		// --- Act ---
		output, err := executeCommand(rootCmd, "check", "--config", configPath)

		// --- Assert ---
		assert.Error(t, err)
		assert.Contains(t, output, "Some checks failed")
		assert.Contains(t, output, "❌ Git: git binary not found in PATH")
	})

	t.Run("docker daemon unreachable", func(t *testing.T) {
		mockClient, configPath := setupCheckTest(t)
		// --- Arrange ---
		require.NoError(t, os.WriteFile(configPath, []byte("agent_provider: test"), 0600))
		mockClient.checkDaemonFunc = func(ctx context.Context) error { return errors.New("daemon unreachable") }

		// --- Act ---
		output, err := executeCommand(rootCmd, "check", "--config", configPath)

		// --- Assert ---
		assert.Error(t, err)
		assert.Contains(t, output, "Some checks failed")
		assert.Contains(t, output, "❌ Docker daemon is not reachable")
	})

	t.Run("missing docker image with fix", func(t *testing.T) {
		mockClient, configPath := setupCheckTest(t)
		// --- Arrange ---
		require.NoError(t, os.WriteFile(configPath, []byte("agent_provider: test"), 0600))
		var pullCalled bool
		mockClient.checkImageFunc = func(ctx context.Context, ref string) (bool, error) { return false, nil }
		mockClient.pullImageFunc = func(ctx context.Context, ref string) error {
			pullCalled = true
			return nil
		}

		// --- Act ---
		output, err := executeCommand(rootCmd, "check", "--config", configPath, "--fix")

		// --- Assert ---
		assert.Error(t, err) // Still fails overall, but fix is attempted
		assert.Contains(t, output, "🔧 Attempting to pull missing images...")
		assert.Contains(t, output, "✅ Auto-fix successfully pulled ubuntu:latest")
		assert.True(t, pullCalled, "PullImage should have been called")
	})

	t.Run("missing config with fix", func(t *testing.T) {
		// This test needs special setup to isolate the home directory
		tmpHome := t.TempDir()
		t.Setenv("HOME", tmpHome)
		viper.Reset() // Ensure viper doesn't use any cached config paths

		// We still need a mock docker client
		mockClient := &mockDockerClient{}
		originalNewDockerClient := newDockerClient
		newDockerClient = func(component string) (dockerChecker, error) {
			return mockClient, nil
		}
		t.Cleanup(func() { newDockerClient = originalNewDockerClient })
		// --- Act ---
		// We don't pass --config, forcing the app to check default locations
		output, err := executeCommand(rootCmd, "check", "--fix")
		// --- Assert ---
		assert.Error(t, err) // Fails on other checks, which is fine
		assert.Contains(t, output, "🔧 Attempting to fix missing config file...")
		assert.Contains(t, output, "✅ Config fixed (created default)")
		// Verify the file was created in our isolated temp home directory
		_, statErr := os.Stat(tmpHome + "/.recac/config.yaml")
		assert.NoError(t, statErr)
	})
}
