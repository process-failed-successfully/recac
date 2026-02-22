package cmdutils

import (
	"context"
	"errors"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/stretchr/testify/assert"
)

// Mock DockerClient
type MockDoctorDockerClient struct {
	PingFunc func(ctx context.Context) (types.Ping, error)
}

func (m *MockDoctorDockerClient) Ping(ctx context.Context) (types.Ping, error) {
	if m.PingFunc != nil {
		return m.PingFunc(ctx)
	}
	return types.Ping{}, nil
}

func TestGetDoctor(t *testing.T) {
	// Backup original functions
	origExecLookPath := execLookPath
	origClientNewClientWithOpts := clientNewClientWithOpts
	origViperConfigFileUsed := viperConfigFileUsed
	origCheckDockerConnectivity := checkDockerConnectivity

	defer func() {
		execLookPath = origExecLookPath
		clientNewClientWithOpts = origClientNewClientWithOpts
		viperConfigFileUsed = origViperConfigFileUsed
		checkDockerConnectivity = origCheckDockerConnectivity
	}()

	t.Run("All checks pass", func(t *testing.T) {
		// Mock Config
		viperConfigFileUsed = func() string {
			return "/home/user/.recac.yaml"
		}

		// Mock Dependencies
		execLookPath = func(file string) (string, error) {
			return "/usr/bin/" + file, nil
		}

		// Mock Docker Connectivity
		checkDockerConnectivity = func(cli DockerClient, err error) (string, bool) {
			return "[✔] Docker: Daemon is responsive\n", true
		}

		output, passed := GetDoctor()

		assert.True(t, passed)
		assert.Contains(t, output, "[✔] Configuration: /home/user/.recac.yaml found")
		assert.Contains(t, output, "[✔] Dependency: git found")
		assert.Contains(t, output, "[✔] Dependency: docker found")
		assert.Contains(t, output, "[✔] Docker: Daemon is responsive")
	})

	t.Run("All checks fail", func(t *testing.T) {
		// Mock Config
		viperConfigFileUsed = func() string {
			return ""
		}

		// Mock Dependencies
		execLookPath = func(file string) (string, error) {
			return "", errors.New("not found")
		}

		// Mock Docker
		checkDockerConnectivity = func(cli DockerClient, err error) (string, bool) {
			return "[✖] Docker: Failed to create client\n", false
		}

		output, passed := GetDoctor()

		assert.False(t, passed)
		assert.Contains(t, output, "[✖] Configuration: Missing config file")
		assert.Contains(t, output, "[✖] Dependency: git not found")
		assert.Contains(t, output, "[✖] Dependency: docker not found")
		assert.Contains(t, output, "[✖] Docker: Failed to create client")
	})
}

func TestCheckDockerConnectivity(t *testing.T) {
	t.Run("Client Creation Error", func(t *testing.T) {
		msg, passed := checkDockerConnectivityFunc(nil, errors.New("creation failed"))
		assert.False(t, passed)
		assert.Contains(t, msg, "Failed to create client")
	})

	t.Run("Ping Success", func(t *testing.T) {
		mockCli := &MockDoctorDockerClient{
			PingFunc: func(ctx context.Context) (types.Ping, error) {
				return types.Ping{}, nil
			},
		}
		msg, passed := checkDockerConnectivityFunc(mockCli, nil)
		assert.True(t, passed)
		assert.Contains(t, msg, "Daemon is responsive")
	})

	t.Run("Ping Failure - Daemon not running", func(t *testing.T) {
		mockCli := &MockDoctorDockerClient{
			PingFunc: func(ctx context.Context) (types.Ping, error) {
				return types.Ping{}, errors.New("Is the docker daemon running?")
			},
		}
		msg, passed := checkDockerConnectivityFunc(mockCli, nil)
		assert.False(t, passed)
		assert.Contains(t, msg, "Daemon not running or socket permission error")
	})

	t.Run("Ping Failure - Other error", func(t *testing.T) {
		mockCli := &MockDoctorDockerClient{
			PingFunc: func(ctx context.Context) (types.Ping, error) {
				return types.Ping{}, errors.New("some other error")
			},
		}
		msg, passed := checkDockerConnectivityFunc(mockCli, nil)
		assert.False(t, passed)
		assert.Contains(t, msg, "Failed to ping daemon")
	})
}
