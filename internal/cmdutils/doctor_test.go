package cmdutils

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"testing"

	"recac/internal/agent"
	"recac/internal/jira"
	"github.com/docker/docker/api/types"
	"github.com/stretchr/testify/assert"
	"github.com/spf13/viper"
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
type MockAgent struct {
	SendFunc func(ctx context.Context, prompt string) (string, error)
	SendStreamFunc func(ctx context.Context, prompt string, onChunk func(string)) (string, error)
}

func (m *MockAgent) Send(ctx context.Context, prompt string) (string, error) {
	if m.SendFunc != nil {
		return m.SendFunc(ctx, prompt)
	}
	return "ok", nil
}

func (m *MockAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	if m.SendStreamFunc != nil {
		return m.SendStreamFunc(ctx, prompt, onChunk)
	}
	return "ok", nil
}

// Mock HTTP Transport for Jira
type MockRoundTripper struct {
	RoundTripFunc func(req *http.Request) (*http.Response, error)
}

func (m *MockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if m.RoundTripFunc != nil {
		return m.RoundTripFunc(req)
	}
	return &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(bytes.NewBufferString("{}")),
	}, nil
}

func TestGetDoctor(t *testing.T) {
	// Backup original functions
	origExecLookPath := execLookPath
	origViperConfigFileUsed := viperConfigFileUsed
	origNewDockerClient := newDockerClient
	origGetJiraClient := GetJiraClient
	origGetAgentClient := GetAgentClient

	defer func() {
		execLookPath = origExecLookPath
		viperConfigFileUsed = origViperConfigFileUsed
		newDockerClient = origNewDockerClient
		GetJiraClient = origGetJiraClient
		GetAgentClient = origGetAgentClient
		viper.Reset()
	}()

	t.Run("All checks pass", func(t *testing.T) {
		// Mock Config
		viperConfigFileUsed = func() string {
			return "/home/user/.recac.yaml"
		}
		viper.Set("provider", "gemini")
		viper.Set("model", "gemini-pro")

		// Mock Dependencies
		execLookPath = func(file string) (string, error) {
			if file == "kubectl" {
				return "", errors.New("not found")
			}
			return "/usr/bin/" + file, nil
		}

		// Mock Docker
		newDockerClient = func() (DockerClient, error) {
			return &MockDoctorDockerClient{
				PingFunc: func(ctx context.Context) (types.Ping, error) {
					return types.Ping{}, nil
				},
			}, nil
		}

		// Mock Jira
		GetJiraClient = func(ctx context.Context) (*jira.Client, error) {
			client := jira.NewClient("https://jira.example.com", "user", "token")
			client.HTTPClient.Transport = &MockRoundTripper{
				RoundTripFunc: func(req *http.Request) (*http.Response, error) {
					// Mock Authenticate call
					if req.URL.Path == "/rest/api/3/myself" {
						return &http.Response{
							StatusCode: 200,
							Body:       io.NopCloser(bytes.NewBufferString(`{"displayName": "Test User"}`)),
						}, nil
					}
					return &http.Response{StatusCode: 404, Body: io.NopCloser(bytes.NewBufferString(""))}, nil
				},
			}
			return client, nil
		}

		// Mock Agent
		GetAgentClient = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
			return &MockAgent{
				SendFunc: func(ctx context.Context, prompt string) (string, error) {
					return "I am ready", nil
				},
			}, nil
		}

		output := GetDoctor()

		// Verify output contains success markers
		// Use partial matches since ANSI codes might be present
		assert.Contains(t, output, "Configuration")
		assert.Contains(t, output, "Found at /home/user/.recac.yaml")
		assert.Contains(t, output, "Dependency: git")
		assert.Contains(t, output, "Found at /usr/bin/git")
		assert.Contains(t, output, "Dependency: docker")
		assert.Contains(t, output, "Found at /usr/bin/docker")
		assert.Contains(t, output, "Daemon is responsive")
		assert.Contains(t, output, "Connected as user")
		assert.Contains(t, output, "Connected to gemini")
	})

	t.Run("All checks fail", func(t *testing.T) {
		// Mock Config
		viperConfigFileUsed = func() string {
			return ""
		}
		viper.Set("provider", "gemini")

		// Mock Dependencies
		execLookPath = func(file string) (string, error) {
			return "", errors.New("not found")
		}

		// Mock Docker
		newDockerClient = func() (DockerClient, error) {
			return &MockDoctorDockerClient{
				PingFunc: func(ctx context.Context) (types.Ping, error) {
					return types.Ping{}, errors.New("connection failed")
				},
			}, nil
		}

		// Mock Jira
		GetJiraClient = func(ctx context.Context) (*jira.Client, error) {
			return nil, errors.New("missing config")
		}

		// Mock Agent
		GetAgentClient = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
			return nil, errors.New("api key missing")
		}

		output := GetDoctor()

		assert.Contains(t, output, "Config file not found")
		assert.Contains(t, output, "Not found in PATH")
		assert.Contains(t, output, "connection failed")
		assert.Contains(t, output, "Not configured")
		assert.Contains(t, output, "Failed to initialize client")
	})
}
