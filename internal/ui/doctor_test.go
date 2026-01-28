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
	CheckImageErr  error
	CheckImageBool bool
}

func (m *MockDockerClient) CheckDaemon(ctx context.Context) error {
	return m.CheckDaemonErr
}

func (m *MockDockerClient) CheckSocket(ctx context.Context) error {
	return m.CheckSocketErr
}

func (m *MockDockerClient) CheckImage(ctx context.Context, imageRef string) (bool, error) {
	return m.CheckImageBool, m.CheckImageErr
}

func (m *MockDockerClient) Close() error {
	return nil
}

func TestGetDoctor(t *testing.T) {
	// Backup and restore original functions to ensure test isolation
	setup := func(t *testing.T) func() {
		originalExecLookPath := execLookPath
		originalDockerClientFactory := dockerClientFactory
		originalViperConfigFileUsed := viperConfigFileUsed
		originalCheckDockerConnectivity := checkDockerConnectivity
		originalCheckDiskSpaceFunc := checkDiskSpaceFunc

		return func() {
			execLookPath = originalExecLookPath
			dockerClientFactory = originalDockerClientFactory
			viperConfigFileUsed = originalViperConfigFileUsed
			checkDockerConnectivity = originalCheckDockerConnectivity
			checkDiskSpaceFunc = originalCheckDiskSpaceFunc
		}
	}

	t.Run("All checks pass", func(t *testing.T) {
		teardown := setup(t)
		defer teardown()

		viperConfigFileUsed = func() string { return "/etc/recac/config.yaml" }
		execLookPath = func(file string) (string, error) {
			return fmt.Sprintf("/usr/bin/%s", file), nil
		}
		dockerClientFactory = func() (DockerClient, error) {
			return &MockDockerClient{CheckImageBool: true}, nil
		}
		checkDiskSpaceFunc = func() (bool, error) {
			return true, nil
		}
		// We use the real checkDockerConnectivityFunc to test the output string building logic too,
		// relying on the MockDockerClient to control behavior.
		checkDockerConnectivity = checkDockerConnectivityFunc

		output := GetDoctor()

		assert.Contains(t, output, "RECAC Doctor")
		assert.Contains(t, output, "[✔] Configuration: /etc/recac/config.yaml found")
		assert.Contains(t, output, "[✔] Dependency: git found in PATH")
		assert.Contains(t, output, "[✔] Dependency: docker found in PATH")
		assert.Contains(t, output, "[✔] Disk Space: Sufficient disk space available")
		assert.Contains(t, output, "[✔] Docker: Daemon is responsive")
		assert.Contains(t, output, "[✔] Docker: Socket is accessible")
		assert.Contains(t, output, "[✔] Docker: Required image ghcr.io/process-failed-successfully/recac-agent:latest is present")
	})

	t.Run("Missing config file", func(t *testing.T) {
		teardown := setup(t)
		defer teardown()

		viperConfigFileUsed = func() string { return "" }
		execLookPath = func(file string) (string, error) { return "/bin/true", nil }
		checkDiskSpaceFunc = func() (bool, error) { return true, nil }
		checkDockerConnectivity = func(cli DockerClient, err error) string { return "" }
		dockerClientFactory = func() (DockerClient, error) { return &MockDockerClient{}, nil }

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
		dockerClientFactory = func() (DockerClient, error) { return &MockDockerClient{}, nil }
		checkDiskSpaceFunc = func() (bool, error) { return true, nil }
		checkDockerConnectivity = func(cli DockerClient, err error) string { return "" }

		output := GetDoctor()
		assert.Contains(t, output, "[✖] Dependency: git not found in PATH")
	})

	t.Run("Low disk space", func(t *testing.T) {
		teardown := setup(t)
		defer teardown()

		viperConfigFileUsed = func() string { return "config" }
		execLookPath = func(file string) (string, error) { return "/bin/true", nil }
		dockerClientFactory = func() (DockerClient, error) { return &MockDockerClient{}, nil }
		checkDockerConnectivity = func(cli DockerClient, err error) string { return "" }

		checkDiskSpaceFunc = func() (bool, error) {
			return false, nil
		}

		output := GetDoctor()
		assert.Contains(t, output, "[✖] Disk Space: Low disk space detected (< 1GB free)")
	})
}

func TestCheckDockerConnectivity(t *testing.T) {
	testCases := []struct {
		name           string
		cli            DockerClient
		err            error
		expectedSubStr []string
	}{
		{
			name: "All pass",
			cli: &MockDockerClient{
				CheckDaemonErr: nil,
				CheckSocketErr: nil,
				CheckImageBool: true,
				CheckImageErr:  nil,
			},
			err: nil,
			expectedSubStr: []string{
				"[✔] Docker: Daemon is responsive",
				"[✔] Docker: Socket is accessible",
				"[✔] Docker: Required image ghcr.io/process-failed-successfully/recac-agent:latest is present",
			},
		},
		{
			name: "Daemon fails",
			cli: &MockDockerClient{
				CheckDaemonErr: errors.New("Is the docker daemon running?"),
			},
			err: nil,
			expectedSubStr: []string{
				"[✖] Docker: Daemon not running or socket permission error",
			},
		},
		{
			name: "Socket fails",
			cli: &MockDockerClient{
				CheckDaemonErr: nil,
				CheckSocketErr: errors.New("socket error"),
			},
			err: nil,
			expectedSubStr: []string{
				"[✔] Docker: Daemon is responsive",
				"[✖] Docker: Socket is not accessible: socket error",
			},
		},
		{
			name: "Image missing",
			cli: &MockDockerClient{
				CheckDaemonErr: nil,
				CheckSocketErr: nil,
				CheckImageBool: false,
				CheckImageErr:  nil,
			},
			err: nil,
			expectedSubStr: []string{
				"[✔] Docker: Daemon is responsive",
				"[✔] Docker: Socket is accessible",
				"[✖] Docker: Required image ghcr.io/process-failed-successfully/recac-agent:latest is missing",
			},
		},
		{
			name: "Image check error",
			cli: &MockDockerClient{
				CheckDaemonErr: nil,
				CheckSocketErr: nil,
				CheckImageBool: false,
				CheckImageErr:  errors.New("image error"),
			},
			err: nil,
			expectedSubStr: []string{
				"[✔] Docker: Daemon is responsive",
				"[✔] Docker: Socket is accessible",
				"[✖] Docker: Error checking image",
				"image error",
			},
		},
		{
			name: "Client creation fails",
			cli:  nil,
			err:  errors.New("client creation error"),
			expectedSubStr: []string{
				"[✖] Docker: Failed to create client: client creation error",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			output := checkDockerConnectivityFunc(tc.cli, tc.err)
			for _, sub := range tc.expectedSubStr {
				assert.Contains(t, output, sub)
			}
		})
	}
}
