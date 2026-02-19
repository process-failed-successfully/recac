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
		// We mock the variable checkDockerConnectivity to simplify,
		// OR we mock clientNewClientWithOpts to return a mocked client.
		// Since checkDockerConnectivity takes a DockerClient, we can test it separately.
		// But GetDoctor calls clientNewClientWithOpts.
		// We cannot easily return a MockDoctorDockerClient from clientNewClientWithOpts because it expects *client.Client.
		// Wait, checkDockerConnectivity takes `DockerClient` interface.
		// But `GetDoctor` calls `clientNewClientWithOpts` which returns `*client.Client`.
		// `*client.Client` implements `DockerClient` interface.
		// We can't mock `clientNewClientWithOpts` to return our mock struct unless our mock struct is *client.Client (which is not possible).

		// So we should mock `checkDockerConnectivity` instead, which is what GetDoctor calls.
		checkDockerConnectivity = func(cli DockerClient, err error) string {
			return "[✔] Docker: Daemon is responsive\n"
		}

		// Mock clientNewClientWithOpts to just return nil (error doesn't matter since we mocked checkDockerConnectivity)
		// But clientNewClientWithOpts return type is (*client.Client, error).
		// We can't return nil, nil if we want to be safe, but GetDoctor passes the result to checkDockerConnectivity.
		// We just need to ensure clientNewClientWithOpts doesn't panic.
		// We can't mock clientNewClientWithOpts easily because of strict return types in Go if we want to return our mock.
		// BUT, we mocked `checkDockerConnectivity`, so `GetDoctor` will use OUR function.
		// We just need `clientNewClientWithOpts` to return something that matches the signature.
		// We can leave `clientNewClientWithOpts` as is, it will return a real client or error.
		// But we want to avoid side effects (like connecting to real docker).
		// Wait, `client.NewClientWithOpts` creates a client struct but doesn't necessarily connect immediately until Ping is called.
		// So it might be fine.
		// Or we can set `clientNewClientWithOpts` to return (nil, nil) and handle nil in our mocked `checkDockerConnectivity`.

		// Actually, let's mock `clientNewClientWithOpts` to return error to ensure we control it.
		// clientNewClientWithOpts = func(...) ...
		// But defining the function signature with external types `client.Client` might be verbose.
		// Let's rely on `checkDockerConnectivity` mock.

		output := GetDoctor()

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
		checkDockerConnectivity = func(cli DockerClient, err error) string {
			return "[✖] Docker: Failed to create client\n"
		}

		output := GetDoctor()

		assert.Contains(t, output, "[✖] Configuration: Missing config file")
		assert.Contains(t, output, "[✖] Dependency: git not found")
		assert.Contains(t, output, "[✖] Dependency: docker not found")
		assert.Contains(t, output, "[✖] Docker: Failed to create client")
	})
}

func TestCheckDockerConnectivity(t *testing.T) {
	t.Run("Client Creation Error", func(t *testing.T) {
		msg := checkDockerConnectivityFunc(nil, errors.New("creation failed"))
		assert.Contains(t, msg, "Failed to create client")
	})

	t.Run("Ping Success", func(t *testing.T) {
		mockCli := &MockDoctorDockerClient{
			PingFunc: func(ctx context.Context) (types.Ping, error) {
				return types.Ping{}, nil
			},
		}
		msg := checkDockerConnectivityFunc(mockCli, nil)
		assert.Contains(t, msg, "Daemon is responsive")
	})

	t.Run("Ping Failure - Daemon not running", func(t *testing.T) {
		mockCli := &MockDoctorDockerClient{
			PingFunc: func(ctx context.Context) (types.Ping, error) {
				return types.Ping{}, errors.New("Is the docker daemon running?")
			},
		}
		msg := checkDockerConnectivityFunc(mockCli, nil)
		assert.Contains(t, msg, "Daemon not running or socket permission error")
	})

	t.Run("Ping Failure - Other error", func(t *testing.T) {
		mockCli := &MockDoctorDockerClient{
			PingFunc: func(ctx context.Context) (types.Ping, error) {
				return types.Ping{}, errors.New("some other error")
			},
		}
		msg := checkDockerConnectivityFunc(mockCli, nil)
		assert.Contains(t, msg, "Failed to ping daemon")
	})
}
