package ui

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

// mockDockerClient is a mock implementation of the DockerClient interface.
type mockDockerClient struct {
	pingFunc func(ctx context.Context) (types.Ping, error)
}

func (m *mockDockerClient) Ping(ctx context.Context) (types.Ping, error) {
	if m.pingFunc != nil {
		return m.pingFunc(ctx)
	}
	return types.Ping{}, nil
}

func TestCheckConfig(t *testing.T) {
	t.Run("Config found", func(t *testing.T) {
		// Set a dummy config file for viper to find
		viper.SetConfigFile("/fake/home/.recac.yaml")
		output := checkConfig()
		assert.Equal(t, "[✔] Configuration: /fake/home/.recac.yaml found\n", output)
		viper.Reset() // Reset viper for other tests
	})

	t.Run("Config not found", func(t *testing.T) {
		// Ensure no config file is set
		viper.Reset()
		output := checkConfig()
		assert.Equal(t, "[✖] Configuration: Missing config file\n", output)
	})
}

func TestCheckDependencies(t *testing.T) {
	originalFunc := execLookPath
	defer func() { execLookPath = originalFunc }()

	t.Run("All dependencies found", func(t *testing.T) {
		execLookPath = func(file string) (string, error) {
			return fmt.Sprintf("/usr/bin/%s", file), nil
		}
		output := checkDependencies()
		assert.Contains(t, output, "[✔] Dependency: git found in PATH")
		assert.Contains(t, output, "[✔] Dependency: docker found in PATH")
	})

	t.Run("Some dependencies missing", func(t *testing.T) {
		execLookPath = func(file string) (string, error) {
			if file == "git" {
				return "", exec.ErrNotFound
			}
			return "/usr/bin/docker", nil
		}
		output := checkDependencies()
		assert.Contains(t, output, "[✖] Dependency: git not found in PATH")
		assert.Contains(t, output, "[✔] Dependency: docker found in PATH")
	})
}

func TestCheckDockerConnectivity(t *testing.T) {
	t.Run("Docker client creation fails", func(t *testing.T) {
		err := errors.New("daemon not available")
		output := checkDockerConnectivity(nil, err)
		assert.Equal(t, "[✖] Docker: Failed to create client: daemon not available\n", output)
	})

	t.Run("Docker ping successful", func(t *testing.T) {
		mockClient := &mockDockerClient{
			pingFunc: func(ctx context.Context) (types.Ping, error) {
				return types.Ping{}, nil
			},
		}
		output := checkDockerConnectivity(mockClient, nil)
		assert.Equal(t, "[✔] Docker: Daemon is responsive\n", output)
	})

	t.Run("Docker ping fails", func(t *testing.T) {
		mockClient := &mockDockerClient{
			pingFunc: func(ctx context.Context) (types.Ping, error) {
				return types.Ping{}, errors.New("ping error")
			},
		}
		output := checkDockerConnectivity(mockClient, nil)
		assert.Equal(t, "[✖] Docker: Failed to ping daemon: ping error\n", output)
	})

	t.Run("Docker daemon not running error", func(t *testing.T) {
		mockClient := &mockDockerClient{
			pingFunc: func(ctx context.Context) (types.Ping, error) {
				return types.Ping{}, errors.New("Is the docker daemon running?")
			},
		}
		output := checkDockerConnectivity(mockClient, nil)
		assert.Equal(t, "[✖] Docker: Daemon not running or socket permission error\n", output)
	})
}
