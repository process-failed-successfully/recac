package ui

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
)

// MockDockerClient is a mock implementation of the DockerClient interface for testing.
type MockDockerClient struct {
	CheckDaemonErr error
	CheckSocketErr error
}

func (m *MockDockerClient) CheckDaemon(ctx context.Context) error {
	return m.CheckDaemonErr
}

func (m *MockDockerClient) CheckSocket(ctx context.Context) error {
	return m.CheckSocketErr
}

func (m *MockDockerClient) Close() error {
	return nil
}

func TestGetDoctor(t *testing.T) {
	// Backup and restore original functions to ensure test isolation
	setup := func(t *testing.T) func() {
		originalExecLookPath := execLookPath
		originalNewDockerClient := newDockerClient
		originalViperConfigFileUsed := viperConfigFileUsed
		originalCheckDockerConnectivity := checkDockerConnectivity

		return func() {
			execLookPath = originalExecLookPath
			newDockerClient = originalNewDockerClient
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
		newDockerClient = func(project string) (DockerClient, error) {
			return &MockDockerClient{}, nil
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
		newDockerClient = func(project string) (DockerClient, error) {
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
			name:           "All checks pass",
			cli:            &MockDockerClient{},
			err:            nil,
			expectedOutput: "[✔] Docker: Daemon is responsive\n",
		},
		{
			name:           "Daemon check fails",
			cli:            &MockDockerClient{CheckDaemonErr: errors.New("daemon unreachable")},
			err:            nil,
			expectedOutput: "[✖] Docker: Daemon not reachable: daemon unreachable\n",
		},
		{
			name:           "Socket check fails",
			cli:            &MockDockerClient{CheckSocketErr: errors.New("socket inaccessible")},
			err:            nil,
			expectedOutput: "[✖] Docker: Socket not accessible: socket inaccessible\n",
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
