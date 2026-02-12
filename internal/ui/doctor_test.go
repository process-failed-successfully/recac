package ui

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os/exec"
	"testing"

	"recac/internal/agent"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/stretchr/testify/assert"
)

// DoctorMockDockerClient is a mock implementation of the DockerClient interface for testing.
type DoctorMockDockerClient struct {
	PingErr error
}

func (m *DoctorMockDockerClient) Ping(ctx context.Context) (types.Ping, error) {
	if m.PingErr != nil {
		return types.Ping{}, m.PingErr
	}
	return types.Ping{}, nil
}

// DoctorMockJiraClient
type DoctorMockJiraClient struct {
	AuthenticateErr error
}

func (m *DoctorMockJiraClient) Authenticate(ctx context.Context) error {
	return m.AuthenticateErr
}

// DoctorMockAgent
type DoctorMockAgent struct {
	SendErr error
}

func (m *DoctorMockAgent) Send(ctx context.Context, prompt string) (string, error) {
	if m.SendErr != nil {
		return "", m.SendErr
	}
	return "ok", nil
}

func (m *DoctorMockAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	if m.SendErr != nil {
		return "", m.SendErr
	}
	onChunk("ok")
	return "ok", nil
}

func TestGetDoctor(t *testing.T) {
	// Backup and restore original functions to ensure test isolation
	setup := func(t *testing.T) func() {
		originalExecLookPath := execLookPath
		originalClientNewClientWithOpts := clientNewClientWithOpts
		originalViperConfigFileUsed := viperConfigFileUsed
		originalCheckDockerConnectivity := checkDockerConnectivity
		originalNewAgentFunc := newAgentFunc
		originalNewJiraClientFunc := newJiraClientFunc
		originalHttpHeadFunc := httpHeadFunc
		originalRunCommand := runCommand
		originalViperGetString := viperGetString

		return func() {
			execLookPath = originalExecLookPath
			clientNewClientWithOpts = originalClientNewClientWithOpts
			viperConfigFileUsed = originalViperConfigFileUsed
			checkDockerConnectivity = originalCheckDockerConnectivity
			newAgentFunc = originalNewAgentFunc
			newJiraClientFunc = originalNewJiraClientFunc
			httpHeadFunc = originalHttpHeadFunc
			runCommand = originalRunCommand
			viperGetString = originalViperGetString
		}
	}

	t.Run("All checks pass", func(t *testing.T) {
		teardown := setup(t)
		defer teardown()

		// Mock Config
		viperGetString = func(key string) string {
			switch key {
			case "agent_provider":
				return "openai"
			case "agent_model":
				return "gpt-4"
			case "api_key":
				return "sk-123"
			case "jira_url":
				return "https://jira.example.com"
			case "jira_email":
				return "user@example.com"
			case "jira_token":
				return "token"
			}
			return ""
		}
		viperConfigFileUsed = func() string { return "/etc/recac/config.yaml" }

		// Mock Dependencies
		execLookPath = func(file string) (string, error) {
			return fmt.Sprintf("/usr/bin/%s", file), nil
		}

		// Mock Docker
		clientNewClientWithOpts = func(ops ...client.Opt) (*client.Client, error) {
			return &client.Client{}, nil
		}
		checkDockerConnectivity = func(cli DockerClient, err error) string {
			return "[✔] Docker: Daemon is responsive\n"
		}

		// Mock LLM
		newAgentFunc = func(provider, apiKey, model, workDir, project string) (agent.Agent, error) {
			return &DoctorMockAgent{}, nil
		}

		// Mock Jira
		newJiraClientFunc = func(url, user, token string) JiraClient {
			return &DoctorMockJiraClient{}
		}

		// Mock Network
		httpHeadFunc = func(url string) (*http.Response, error) {
			return &http.Response{StatusCode: 200}, nil
		}

		// Mock Git Config
		runCommand = func(name string, args ...string) (string, error) {
			if name == "git" && args[0] == "config" {
				return "configured-value", nil
			}
			return "", nil
		}

		output := GetDoctor()

		assert.Contains(t, output, "RECAC Doctor")
		assert.Contains(t, output, "[✔] Configuration: /etc/recac/config.yaml found")
		assert.Contains(t, output, "[✔] Dependency: git found in PATH")
		assert.Contains(t, output, "[✔] LLM: Connected to openai")
		assert.Contains(t, output, "[✔] Jira: Authenticated")
		assert.Contains(t, output, "[✔] Network: Internet connectivity OK")
		assert.Contains(t, output, "[✔] Git: user.name=configured-value")
	})

	t.Run("Missing config file", func(t *testing.T) {
		teardown := setup(t)
		defer teardown()

		viperConfigFileUsed = func() string { return "" }
		viperGetString = func(key string) string { return "" }

		// Stub critical functions
		execLookPath = func(file string) (string, error) { return "/bin/true", nil }
		checkDockerConnectivity = func(cli DockerClient, err error) string { return "" }
		newAgentFunc = func(p, k, m, w, pr string) (agent.Agent, error) { return &DoctorMockAgent{}, nil }
		newJiraClientFunc = func(url, user, token string) JiraClient { return &DoctorMockJiraClient{} }
		httpHeadFunc = func(url string) (*http.Response, error) { return nil, nil }
		runCommand = func(name string, args ...string) (string, error) { return "", nil }

		output := GetDoctor()
		assert.Contains(t, output, "[✖] Configuration: Missing config file")
	})

	t.Run("Missing git dependency", func(t *testing.T) {
		teardown := setup(t)
		defer teardown()

		viperConfigFileUsed = func() string { return "config.yaml" }
		viperGetString = func(key string) string { return "" }
		execLookPath = func(file string) (string, error) {
			if file == "git" {
				return "", exec.ErrNotFound
			}
			return "/usr/bin/docker", nil
		}

		// Stub critical functions
		checkDockerConnectivity = func(cli DockerClient, err error) string { return "" }
		newAgentFunc = func(p, k, m, w, pr string) (agent.Agent, error) { return &DoctorMockAgent{}, nil }
		newJiraClientFunc = func(url, user, token string) JiraClient { return &DoctorMockJiraClient{} }
		httpHeadFunc = func(url string) (*http.Response, error) { return nil, nil }
		runCommand = func(name string, args ...string) (string, error) { return "", nil }

		output := GetDoctor()
		assert.Contains(t, output, "[✖] Dependency: git not found in PATH")
	})

	t.Run("LLM Failure", func(t *testing.T) {
		teardown := setup(t)
		defer teardown()

		viperGetString = func(key string) string {
			if key == "agent_provider" { return "openai" }
			if key == "api_key" { return "sk-123" }
			return ""
		}

		newAgentFunc = func(provider, apiKey, model, workDir, project string) (agent.Agent, error) {
			return nil, errors.New("agent creation failed")
		}

		// Stub other checks to avoid panic/noise
		viperConfigFileUsed = func() string { return "" }
		execLookPath = func(file string) (string, error) { return "", nil }
		checkDockerConnectivity = func(cli DockerClient, err error) string { return "" }
		newJiraClientFunc = func(url, user, token string) JiraClient { return &DoctorMockJiraClient{} }
		httpHeadFunc = func(url string) (*http.Response, error) { return nil, nil }
		runCommand = func(name string, args ...string) (string, error) { return "", nil }

		output := GetDoctor()
		assert.Contains(t, output, "[✖] LLM: Failed to initialize agent: agent creation failed")
	})

	t.Run("Jira Failure", func(t *testing.T) {
		teardown := setup(t)
		defer teardown()

		viperGetString = func(key string) string {
			if key == "jira_url" { return "https://jira.example.com" }
			if key == "jira_email" { return "user@example.com" }
			if key == "jira_token" { return "token" }
			return ""
		}

		newJiraClientFunc = func(url, user, token string) JiraClient {
			return &DoctorMockJiraClient{AuthenticateErr: errors.New("401 Unauthorized")}
		}

		// Stub others
		viperConfigFileUsed = func() string { return "" }
		execLookPath = func(file string) (string, error) { return "", nil }
		checkDockerConnectivity = func(cli DockerClient, err error) string { return "" }
		newAgentFunc = func(p, k, m, w, pr string) (agent.Agent, error) { return &DoctorMockAgent{}, nil }
		httpHeadFunc = func(url string) (*http.Response, error) { return nil, nil }
		runCommand = func(name string, args ...string) (string, error) { return "", nil }

		output := GetDoctor()
		assert.Contains(t, output, "[✖] Jira: Authentication failed: 401 Unauthorized")
	})

	t.Run("Network Failure", func(t *testing.T) {
		teardown := setup(t)
		defer teardown()

		viperGetString = func(key string) string { return "" }

		httpHeadFunc = func(url string) (*http.Response, error) {
			return nil, errors.New("timeout")
		}

		// Stub others
		viperConfigFileUsed = func() string { return "" }
		execLookPath = func(file string) (string, error) { return "", nil }
		checkDockerConnectivity = func(cli DockerClient, err error) string { return "" }
		newAgentFunc = func(p, k, m, w, pr string) (agent.Agent, error) { return &DoctorMockAgent{}, nil }
		newJiraClientFunc = func(url, user, token string) JiraClient { return &DoctorMockJiraClient{} }
		runCommand = func(name string, args ...string) (string, error) { return "", nil }

		output := GetDoctor()
		assert.Contains(t, output, "[✖] Network: Failed to reach GitHub: timeout")
	})

	t.Run("Git Config Missing", func(t *testing.T) {
		teardown := setup(t)
		defer teardown()

		viperGetString = func(key string) string { return "" }

		runCommand = func(name string, args ...string) (string, error) {
			return "", errors.New("exit status 1")
		}

		// Stub others
		viperConfigFileUsed = func() string { return "" }
		execLookPath = func(file string) (string, error) { return "", nil }
		checkDockerConnectivity = func(cli DockerClient, err error) string { return "" }
		newAgentFunc = func(p, k, m, w, pr string) (agent.Agent, error) { return &DoctorMockAgent{}, nil }
		newJiraClientFunc = func(url, user, token string) JiraClient { return &DoctorMockJiraClient{} }
		httpHeadFunc = func(url string) (*http.Response, error) { return nil, nil }

		output := GetDoctor()
		assert.Contains(t, output, "[✖] Git: 'user.name' not configured")
		assert.Contains(t, output, "[✖] Git: 'user.email' not configured")
	})
}
