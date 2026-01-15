package ui

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/stretchr/testify/assert"
)

// MockDockerClient is a mock implementation of the DockerClient interface for testing.
type MockDockerClient struct {
	PingErr error
}

func (m *MockDockerClient) Ping(ctx context.Context) (types.Ping, error) {
	return types.Ping{}, m.PingErr
}

func TestGetDoctor(t *testing.T) {
	// Backup and restore original functions to ensure test isolation
	setup := func(t *testing.T) func() {
		originalExecLookPath := execLookPath
		originalClientNewClientWithOpts := clientNewClientWithOpts
		originalViperConfigFileUsed := viperConfigFileUsed
		originalCheckDockerConnectivity := checkDockerConnectivity

		return func() {
			execLookPath = originalExecLookPath
			clientNewClientWithOpts = originalClientNewClientWithOpts
			viperConfigFileUsed = originalViperConfigFileUsed
			checkDockerConnectivity = originalCheckDockerConnectivity
		}
	}

	t.Run("All checks pass", func(t *testing.T) {
		teardown := setup(t)
		defer teardown()

		viperConfigFileUsed = func() string { return "/etc/recac/config.yaml" }
		execLookPath = func(file string) (string, error) {
			return fmt.Sprintf("/usr/bin/%s", file), nil
		}
		clientNewClientWithOpts = func(ops ...client.Opt) (*client.Client, error) {
			return &client.Client{}, nil
		}
		checkDockerConnectivity = func(cli DockerClient, err error) string {
			return "[✔] Docker: Daemon is responsive\n"
		}

		output := GetDoctor()

		assert.Contains(t, output, "RECAC Doctor")
		assert.Contains(t, output, "[✔] Configuration: /etc/recac/config.yaml found")
		assert.Contains(t, output, "[✔] Dependency: git found in PATH")
		assert.Contains(t, output, "[✔] Dependency: docker found in PATH")
		assert.Contains(t, output, "[✔] Docker: Daemon is responsive")
	})

	t.Run("Missing config file", func(t *testing.T) {
		teardown := setup(t)
		defer teardown()

		viperConfigFileUsed = func() string { return "" }
		execLookPath = func(file string) (string, error) { return "/bin/true", nil }
		checkDockerConnectivity = func(cli DockerClient, err error) string { return "" }

		output := GetDoctor()
		assert.Contains(t, output, "[✖] Configuration: Missing config file")
	})

	t.Run("Missing git dependency", func(t *testing.T) {
		teardown := setup(t)
		defer teardown()

		viperConfigFileUsed = func() string { return "config.yaml" }
		execLookPath = func(file string) (string, error) {
			if file == "git" {
				return "", exec.ErrNotFound
			}
			return "/usr/bin/docker", nil
		}
		checkDockerConnectivity = func(cli DockerClient, err error) string { return "" }

		output := GetDoctor()
		assert.Contains(t, output, "[✖] Dependency: git not found in PATH")
	})

	t.Run("Docker client creation fails", func(t *testing.T) {
		teardown := setup(t)
		defer teardown()

		viperConfigFileUsed = func() string { return "config.yaml" }
		execLookPath = func(file string) (string, error) { return "/bin/true", nil }
		clientNewClientWithOpts = func(ops ...client.Opt) (*client.Client, error) {
			return nil, errors.New("docker client error")
		}
		// Use the real implementation of checkDockerConnectivity
		checkDockerConnectivity = checkDockerConnectivityFunc

		output := GetDoctor()
		assert.Contains(t, output, "[✖] Docker: Failed to create client: docker client error")
	})
}

func TestCheckDockerConnectivity(t *testing.T) {
	testCases := []struct {
		name           string
		cli            DockerClient
		err            error
		expectedOutput string
	}{
		{
			name:           "Ping successful",
			cli:            &MockDockerClient{PingErr: nil},
			err:            nil,
			expectedOutput: "[✔] Docker: Daemon is responsive\n",
		},
		{
			name:           "Ping fails with daemon error",
			cli:            &MockDockerClient{PingErr: errors.New("Is the docker daemon running?")},
			err:            nil,
			expectedOutput: "[✖] Docker: Daemon not running or socket permission error\n",
		},
		{
			name:           "Ping fails with other error",
			cli:            &MockDockerClient{PingErr: errors.New("some other error")},
			err:            nil,
			expectedOutput: "[✖] Docker: Failed to ping daemon: some other error\n",
		},
		{
			name:           "Client creation fails",
			cli:            nil,
			err:            errors.New("client creation error"),
			expectedOutput: "[✖] Docker: Failed to create client: client creation error\n",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			output := checkDockerConnectivityFunc(tc.cli, tc.err)
			assert.Equal(t, tc.expectedOutput, output)
		})
	}
}
