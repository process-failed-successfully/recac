package cmdutils

import (
	"context"
	"errors"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/stretchr/testify/assert"
)

// MockDockerClient for doctor tests
type MockDockerClient struct {
	pingFunc func(ctx context.Context) (types.Ping, error)
}

func (m *MockDockerClient) Ping(ctx context.Context) (types.Ping, error) {
	if m.pingFunc != nil {
		return m.pingFunc(ctx)
	}
	return types.Ping{}, nil
}

func TestGetDoctor(t *testing.T) {
	// Backup original functions
	origExecLookPath := execLookPath
	origClientNew := clientNewClientWithOpts
	origViperConfigFile := viperConfigFileUsed
	origCheckDocker := checkDockerConnectivity

	defer func() {
		execLookPath = origExecLookPath
		clientNewClientWithOpts = origClientNew
		viperConfigFileUsed = origViperConfigFile
		checkDockerConnectivity = origCheckDocker
	}()

	t.Run("All Checks Pass", func(t *testing.T) {
		// Mocks
		viperConfigFileUsed = func() string { return "/tmp/config.yaml" }
		execLookPath = func(file string) (string, error) { return "/usr/bin/" + file, nil }

		// checkDockerConnectivity is mocked, so we bypass client logic
		checkDockerConnectivity = func(cli DockerClient, err error) string {
			return "[✔] Docker: Daemon is responsive\n"
		}

		// We mock clientNewClientWithOpts but since we mock checkDockerConnectivity,
		// the return value doesn't matter as long as it fits the signature.
		// However, the signature returns *client.Client which is a struct.
		// We can return nil.
		clientNewClientWithOpts = func(ops ...client.Opt) (*client.Client, error) {
			return nil, nil
		}

		output := GetDoctor()
		assert.Contains(t, output, "[✔] Configuration: /tmp/config.yaml found")
		assert.Contains(t, output, "[✔] Dependency: git found")
		assert.Contains(t, output, "[✔] Dependency: docker found")
		assert.Contains(t, output, "[✔] Docker: Daemon is responsive")
	})

	t.Run("Config Missing", func(t *testing.T) {
		viperConfigFileUsed = func() string { return "" }
		execLookPath = func(file string) (string, error) { return "", nil }
		clientNewClientWithOpts = func(ops ...client.Opt) (*client.Client, error) { return nil, nil }
		checkDockerConnectivity = func(cli DockerClient, err error) string { return "[✖] Docker: Failed" }

		output := GetDoctor()
		assert.Contains(t, output, "[✖] Configuration: Missing config file")
	})

	t.Run("Dependencies Missing", func(t *testing.T) {
		viperConfigFileUsed = func() string { return "/tmp/config.yaml" }
		execLookPath = func(file string) (string, error) { return "", errors.New("not found") }
		clientNewClientWithOpts = func(ops ...client.Opt) (*client.Client, error) { return nil, nil }
		checkDockerConnectivity = func(cli DockerClient, err error) string { return "" }

		output := GetDoctor()
		assert.Contains(t, output, "[✖] Dependency: git not found")
	})
}

func TestCheckDockerConnectivity(t *testing.T) {
	t.Run("Client Creation Error", func(t *testing.T) {
		msg := checkDockerConnectivityFunc(nil, errors.New("client creation failed"))
		assert.Contains(t, msg, "Failed to create client")
	})

	t.Run("Ping Success", func(t *testing.T) {
		mockCli := &MockDockerClient{
			pingFunc: func(ctx context.Context) (types.Ping, error) {
				return types.Ping{}, nil
			},
		}
		msg := checkDockerConnectivityFunc(mockCli, nil)
		assert.Contains(t, msg, "Daemon is responsive")
	})

	t.Run("Ping Daemon Not Running", func(t *testing.T) {
		mockCli := &MockDockerClient{
			pingFunc: func(ctx context.Context) (types.Ping, error) {
				return types.Ping{}, errors.New("Is the docker daemon running?")
			},
		}
		msg := checkDockerConnectivityFunc(mockCli, nil)
		assert.Contains(t, msg, "Daemon not running")
	})

	t.Run("Ping Generic Error", func(t *testing.T) {
		mockCli := &MockDockerClient{
			pingFunc: func(ctx context.Context) (types.Ping, error) {
				return types.Ping{}, errors.New("generic connection error")
			},
		}
		msg := checkDockerConnectivityFunc(mockCli, nil)
		assert.Contains(t, msg, "Failed to ping daemon")
	})
}
