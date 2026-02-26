package cmdutils

import (
	"context"
	"errors"
	"testing"

	"recac/internal/agent"

	"github.com/docker/docker/api/types"
	"github.com/spf13/viper"
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

// Mock Agent
type MockDoctorAgent struct {
	Response string
	Err      error
}

func (m *MockDoctorAgent) Send(ctx context.Context, prompt string) (string, error) {
	return m.Response, m.Err
}

func (m *MockDoctorAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	return m.Response, m.Err
}

func TestGetDoctor(t *testing.T) {
	// Backup original functions
	origExecLookPath := execLookPath
	origClientNewClientWithOpts := clientNewClientWithOpts
	origViperConfigFileUsed := viperConfigFileUsed
	origCheckDockerConnectivity := checkDockerConnectivity
	origNewAgent := newAgent

	defer func() {
		execLookPath = origExecLookPath
		clientNewClientWithOpts = origClientNewClientWithOpts
		viperConfigFileUsed = origViperConfigFileUsed
		checkDockerConnectivity = origCheckDockerConnectivity
		newAgent = origNewAgent
		viper.Reset()
	}()

	t.Run("All checks pass", func(t *testing.T) {
		// Mock Config
		viperConfigFileUsed = func() string {
			return "/home/user/.recac.yaml"
		}
		viper.Set("provider", "test-provider")
		viper.Set("model", "test-model")
		viper.Set("api_key", "test-key")

		// Mock Dependencies
		execLookPath = func(file string) (string, error) {
			return "/usr/bin/" + file, nil
		}

		// Mock Docker Connectivity
		checkDockerConnectivity = func(cli DockerClient, err error) string {
			return "[✔] Docker: Daemon is responsive\n"
		}

		// Mock Agent
		newAgent = func(provider, apiKey, model, workDir, project string) (agent.Agent, error) {
			return &MockDoctorAgent{Response: "I am functional", Err: nil}, nil
		}

		output := GetDoctor()

		assert.Contains(t, output, "[✔] Configuration: /home/user/.recac.yaml found")
		assert.Contains(t, output, "[✔] Dependency: git found")
		assert.Contains(t, output, "[✔] Dependency: docker found")
		assert.Contains(t, output, "[✔] Docker: Daemon is responsive")
		assert.Contains(t, output, "[✔] AI: Connected to test-provider/test-model")
		assert.Contains(t, output, "Response: \"I am functional\"")
	})

	t.Run("All checks fail", func(t *testing.T) {
		// Mock Config
		viperConfigFileUsed = func() string {
			return ""
		}
		viper.Reset() // Clear provider/model settings

		// Mock Dependencies
		execLookPath = func(file string) (string, error) {
			return "", errors.New("not found")
		}

		// Mock Docker
		checkDockerConnectivity = func(cli DockerClient, err error) string {
			return "[✖] Docker: Failed to create client\n"
		}

		// Mock Agent (should not be called if provider is empty, but just in case)
		newAgent = func(provider, apiKey, model, workDir, project string) (agent.Agent, error) {
			return nil, errors.New("should not be called")
		}

		output := GetDoctor()

		assert.Contains(t, output, "[✖] Configuration: Missing config file")
		assert.Contains(t, output, "[✖] Dependency: git not found")
		assert.Contains(t, output, "[✖] Dependency: docker not found")
		assert.Contains(t, output, "[✖] Docker: Failed to create client")
		assert.Contains(t, output, "[?] AI: Provider not configured")
	})

	t.Run("AI Connection Fail", func(t *testing.T) {
		// Setup Success for others
		viperConfigFileUsed = func() string { return "conf" }
		checkDockerConnectivity = func(cli DockerClient, err error) string { return "" }
		execLookPath = func(file string) (string, error) { return "path", nil }

		viper.Set("provider", "broken-provider")

		newAgent = func(provider, apiKey, model, workDir, project string) (agent.Agent, error) {
			return &MockDoctorAgent{Err: errors.New("timeout connecting")}, nil
		}

		output := GetDoctor()
		assert.Contains(t, output, "[✖] AI: Connection failed (broken-provider/): timeout connecting")
	})

	t.Run("AI Init Fail", func(t *testing.T) {
		// Setup Success for others
		viperConfigFileUsed = func() string { return "conf" }
		checkDockerConnectivity = func(cli DockerClient, err error) string { return "" }
		execLookPath = func(file string) (string, error) { return "path", nil }

		viper.Set("provider", "bad-provider")

		newAgent = func(provider, apiKey, model, workDir, project string) (agent.Agent, error) {
			return nil, errors.New("unknown provider")
		}

		output := GetDoctor()
		assert.Contains(t, output, "[✖] AI: Failed to initialize agent (bad-provider/): unknown provider")
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
