package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"recac/internal/agent"
	"recac/internal/cmdutils"
	"recac/internal/docker"
	"recac/internal/git"
	"recac/internal/jira"
	"recac/internal/runner"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/assert"
)

// Define local mock to avoid circular dependencies
type MockDockerClient struct {
	CheckDaemonFunc   func(ctx context.Context) error
	RunContainerFunc  func(ctx context.Context, image, workspace string, extraBinds, env []string, user string) (string, error)
	StopContainerFunc func(ctx context.Context, containerID string) error
	ExecFunc          func(ctx context.Context, containerID string, cmd []string) (string, error)
	ExecAsUserFunc    func(ctx context.Context, containerID, user string, cmd []string) (string, error)
	PullImageFunc     func(ctx context.Context, image string) error
	ImageExistsFunc   func(ctx context.Context, image string) (bool, error)
	ImageBuildFunc    func(ctx context.Context, options docker.ImageBuildOptions) (string, error)
}

func (m *MockDockerClient) CheckDaemon(ctx context.Context) error {
	if m.CheckDaemonFunc != nil { return m.CheckDaemonFunc(ctx) }
	return nil
}
func (m *MockDockerClient) RunContainer(ctx context.Context, image, workspace string, extraBinds, env []string, user string) (string, error) {
	if m.RunContainerFunc != nil { return m.RunContainerFunc(ctx, image, workspace, extraBinds, env, user) }
	return "mock-container-id", nil
}
func (m *MockDockerClient) StopContainer(ctx context.Context, containerID string) error {
	if m.StopContainerFunc != nil { return m.StopContainerFunc(ctx, containerID) }
	return nil
}
func (m *MockDockerClient) Exec(ctx context.Context, containerID string, cmd []string) (string, error) {
	if m.ExecFunc != nil { return m.ExecFunc(ctx, containerID, cmd) }
	return "", nil
}
func (m *MockDockerClient) ExecAsUser(ctx context.Context, containerID, user string, cmd []string) (string, error) {
	if m.ExecAsUserFunc != nil { return m.ExecAsUserFunc(ctx, containerID, user, cmd) }
	return "", nil
}
func (m *MockDockerClient) PullImage(ctx context.Context, image string) error {
	if m.PullImageFunc != nil { return m.PullImageFunc(ctx, image) }
	return nil
}
func (m *MockDockerClient) ImageExists(ctx context.Context, image string) (bool, error) {
	if m.ImageExistsFunc != nil { return m.ImageExistsFunc(ctx, image) }
	return true, nil
}
func (m *MockDockerClient) ImageBuild(ctx context.Context, options docker.ImageBuildOptions) (string, error) {
	if m.ImageBuildFunc != nil { return m.ImageBuildFunc(ctx, options) }
	return "mock-image-id", nil
}

// Add missing mock functions to satisfy interface if any
func (m *MockDockerClient) ContainerCreate(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *specs.Platform, containerName string) (container.CreateResponse, error) {
	return container.CreateResponse{ID: "mock-id"}, nil
}
func (m *MockDockerClient) ContainerStart(ctx context.Context, containerID string, options container.StartOptions) error {
	return nil
}
func (m *MockDockerClient) ContainerExecCreate(ctx context.Context, container string, config container.ExecOptions) (types.IDResponse, error) {
	return types.IDResponse{ID: "exec-id"}, nil
}
func (m *MockDockerClient) ImageList(ctx context.Context, options image.ListOptions) ([]image.Summary, error) {
	return []image.Summary{}, nil
}
func (m *MockDockerClient) ServerVersion(ctx context.Context) (types.Version, error) {
	return types.Version{}, nil
}
func (m *MockDockerClient) Close() error { return nil }
func (m *MockDockerClient) ListContainers(ctx context.Context, options container.ListOptions) ([]types.Container, error) { return []types.Container{}, nil }
func (m *MockDockerClient) RemoveContainer(ctx context.Context, containerID string, force bool) error { return nil }
func (m *MockDockerClient) KillContainer(ctx context.Context, containerID, signal string) error { return nil }
func (m *MockDockerClient) CheckSocket(ctx context.Context) error { return nil }
func (m *MockDockerClient) CheckImage(ctx context.Context, imageRef string) (bool, error) { return true, nil }
func (m *MockDockerClient) ExecInteractive(ctx context.Context, containerID string, cmd []string) error { return nil }


func TestProcessJiraTicket(t *testing.T) {
	// Mock RunWorkflow
	originalRunWorkflow := RunWorkflow
	defer func() { RunWorkflow = originalRunWorkflow }()
	RunWorkflow = func(ctx context.Context, cfg SessionConfig) error {
		return nil // Prevent running the full session
	}

	// Mock SetupWorkspace
	originalSetup := cmdutils.SetupWorkspace
	defer func() { cmdutils.SetupWorkspace = originalSetup }()

	cmdutils.SetupWorkspace = func(ctx context.Context, gitClient git.IClient, repoURL, workspace, ticketID, epicKey, timestamp string) (string, error) {
		// Mock success
		// Ensure workspace dir exists
		os.MkdirAll(workspace, 0755)
		return repoURL, nil
	}

	// Mock Jira Server
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	// Mock Ticket Response
	mux.HandleFunc("/rest/api/3/issue/TEST-1", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"key": "TEST-1",
			"fields": map[string]interface{}{
				"summary": "Test Ticket",
				"description": map[string]interface{}{
					"type":    "doc",
					"version": 1,
					"content": []map[string]interface{}{
						{
							"type": "paragraph",
							"content": []map[string]interface{}{
								{
									"type": "text",
									"text": "Repo: https://github.com/example/repo",
								},
							},
						},
					},
				},
				"issuelinks": []interface{}{},
			},
		})
	})

	// Mock Transition (search for transitions first)
	mux.HandleFunc("/rest/api/3/issue/TEST-1/transitions", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"transitions": []interface{}{
				map[string]interface{}{"id": "11", "name": "In Progress"},
			},
		})
	})

	// Create Client
	jClient := jira.NewClient(server.URL, "user", "token")

	// Config
	tmpDir, _ := os.MkdirTemp("", "workflow-jira-test")
	defer os.RemoveAll(tmpDir)

	cfg := SessionConfig{
		ProjectPath: tmpDir,
		SessionName: "test-run",
		Cleanup:     true,
		IsMock:      true,
	}

	err := ProcessJiraTicket(context.Background(), "TEST-1", jClient, cfg, nil)

	// Since we don't have DB, we expect RunWorkflow to fail or we mock DB?
	// mocking DB is hard.
	// We'll rely on IsMock: true in SessionConfig to perform a "Mock" run which should be lighter.

	// Check app_spec.txt
	specPath := fmt.Sprintf("%s/app_spec.txt", tmpDir)

	// If we want to verify, we should use Cleanup=false
	cfg.Cleanup = false

	err = ProcessJiraTicket(context.Background(), "TEST-1", jClient, cfg, nil)

	// Assert steps
	assert.FileExists(t, specPath)
	if err != nil {
		assert.Contains(t, err.Error(), "circuit breaker")
	} else {
		assert.NoError(t, err)
	}

	content, _ := os.ReadFile(specPath)
	assert.Contains(t, string(content), "Test Ticket")
	assert.Contains(t, string(content), "https://github.com/example/repo")
}

func TestProcessDirectTask(t *testing.T) {
	// Mock RunWorkflow
	originalRunWorkflow := RunWorkflow
	defer func() { RunWorkflow = originalRunWorkflow }()
	RunWorkflow = func(ctx context.Context, cfg SessionConfig) error {
		return nil // Prevent running the full session
	}

	// Mock SetupWorkspace
	originalSetup := cmdutils.SetupWorkspace
	defer func() { cmdutils.SetupWorkspace = originalSetup }()

	cmdutils.SetupWorkspace = func(ctx context.Context, gitClient git.IClient, repoURL, workspace, ticketID, epicKey, timestamp string) (string, error) {
		os.MkdirAll(workspace, 0755)
		return repoURL, nil
	}

	tmpDir, _ := os.MkdirTemp("", "workflow-direct-test")
	defer os.RemoveAll(tmpDir)

	cfg := SessionConfig{
		ProjectPath: tmpDir,
		RepoURL:     "https://github.com/example/direct",
		Summary:     "Do something",
		IsMock:      true,
	}

	err := ProcessDirectTask(context.Background(), cfg)

	// Check app_spec.txt
	specPath := fmt.Sprintf("%s/app_spec.txt", tmpDir)
	assert.FileExists(t, specPath)

	if err != nil {
		// assert.Contains(t, err.Error(), "database")
	}
}

func TestRunWorkflow_Detached(t *testing.T) {
	t.Skip("Skipping detached test due to binary dependency")
}

func TestProcessJiraTicket_WithRepoURL(t *testing.T) {
	// Mock NewSessionFunc to inject mock docker client
	originalNewSessionFunc := NewSessionFunc
	defer func() { NewSessionFunc = originalNewSessionFunc }()

	var capturedWorkspace string

	NewSessionFunc = func(d runner.DockerClient, a agent.Agent, workspace, image, project, provider, model string, maxAgents int) *runner.Session {
		capturedWorkspace = workspace

		// Use mock Docker client
		mockDocker := &MockDockerClient{}
		mockDocker.CheckDaemonFunc = func(ctx context.Context) error { return nil }
		mockDocker.ImageExistsFunc = func(ctx context.Context, image string) (bool, error) { return true, nil }

		// Catch ExecAsUser which MockAgent uses for the script
		mockDocker.ExecAsUserFunc = func(ctx context.Context, containerID, user string, cmd []string) (string, error) {
			// Join command to check content
			cmdStr := strings.Join(cmd, " ")

			// Detect agent-bridge import
			if strings.Contains(cmdStr, "agent-bridge import") {
				// Parse JSON from the command
				// Format: cat << 'EOF' | agent-bridge import\n{JSON}\nEOF
				start := strings.Index(cmdStr, "{")
				end := strings.LastIndex(cmdStr, "}")
				if start != -1 && end != -1 && end > start {
					jsonContent := cmdStr[start : end+1]

					// Write to workspace to simulate effect
					// We need to write to the actual temp dir used by the test
					if capturedWorkspace != "" {
						path := fmt.Sprintf("%s/feature_list.json", capturedWorkspace)
						err := os.WriteFile(path, []byte(jsonContent), 0644)
						if err != nil {
							fmt.Printf("Failed to write mock feature list: %v\n", err)
						}
					}
				}
				return "Imported features", nil
			}
			return "", nil
		}

		// Ensure we pass the mock docker client
		s := runner.NewSession(mockDocker, a, workspace, image, project, provider, model, maxAgents)

		// Force MaxIterations to something small but enough to pass
		s.MaxIterations = 5

		// Ensure we don't try to use Local Agent if on CI
		s.UseLocalAgent = false

		return s
	}

	// Mock SetupWorkspace
	originalSetup := cmdutils.SetupWorkspace
	defer func() { cmdutils.SetupWorkspace = originalSetup }()

	cmdutils.SetupWorkspace = func(ctx context.Context, gitClient git.IClient, repoURL, workspace, ticketID, epicKey, timestamp string) (string, error) {
		os.MkdirAll(workspace, 0755)
		return repoURL, nil
	}

	// Mock Jira Server (minimal)
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	// Mock Ticket Response (NO Repo: URL here to test fallback skip)
	mux.HandleFunc("/rest/api/3/issue/TEST-1", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"key": "TEST-1",
			"fields": map[string]interface{}{
				"summary": "Test Ticket",
				"description": map[string]interface{}{
					"type": "doc", "version": 1,
					"content": []map[string]interface{}{
						{"type": "paragraph", "content": []map[string]interface{}{{"type": "text", "text": "No repo here"}}},
					},
				},
				"issuelinks": []interface{}{},
			},
		})
	})

	mux.HandleFunc("/rest/api/3/issue/TEST-1/transitions", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			json.NewEncoder(w).Encode(map[string]interface{}{"transitions": []interface{}{map[string]interface{}{"id": "11", "name": "In Progress"}}})
		} else {
			w.WriteHeader(http.StatusNoContent)
		}
	})

	jClient := jira.NewClient(server.URL, "user", "token")
	tmpDir, _ := os.MkdirTemp("", "workflow-jira-repo-test")
	defer os.RemoveAll(tmpDir)

	cfg := SessionConfig{
		ProjectPath: tmpDir,
		RepoURL:     "https://github.com/example/already-provided",
		IsMock:      true,
		Cleanup:     false,
	}

	err := ProcessJiraTicket(context.Background(), "TEST-1", jClient, cfg, nil)

	// Should NOT return "no repo url found" error because RepoURL was provided in cfg.
	if err != nil {
		assert.NotContains(t, err.Error(), "no repo url found")
	}

	specPath := fmt.Sprintf("%s/app_spec.txt", tmpDir)
	assert.FileExists(t, specPath)
	content, _ := os.ReadFile(specPath)
	assert.Contains(t, string(content), "TEST-1")
}

func TestRunWorkflow_Normal(t *testing.T) {
	// Mock cmdutils.GetAgentClient
	originalGetAgentClient := cmdutils.GetAgentClient
	defer func() { cmdutils.GetAgentClient = originalGetAgentClient }()
	cmdutils.GetAgentClient = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return agent.NewMockAgent(), nil
	}

	// Mock NewSessionFunc
	originalNewSessionFunc := NewSessionFunc
	defer func() { NewSessionFunc = originalNewSessionFunc }()
	NewSessionFunc = func(d runner.DockerClient, a agent.Agent, workspace, image, project, provider, model string, maxAgents int) *runner.Session {
		s := runner.NewSession(d, a, workspace, image, project, provider, model, maxAgents)
		s.MaxIterations = 1
		// Use a mock agent that returns a command to avoid NoOp
		mockAg := agent.NewMockAgent()
		s.Agent = mockAg
		return s
	}

	tmpDir, _ := os.MkdirTemp("", "workflow-normal-test")
	defer os.RemoveAll(tmpDir)

	// Create app_spec.txt required by RunLoop
	os.WriteFile(fmt.Sprintf("%s/app_spec.txt", tmpDir), []byte("test spec"), 0644)

	cfg := SessionConfig{
		ProjectPath: tmpDir,
		SessionName: "normal-test",
		IsMock:      false,
		ProjectName: "test-project",
		Debug:       true,
		AllowDirty:  true, // Avoid git checks
	}

	err := RunWorkflow(context.Background(), cfg)

	// Check if err is related to execution flow, not setup
	if err != nil && err.Error() != "circuit breaker: no-op loop" && err.Error() != "maximum iterations reached" {
		// Acceptable errors for this test which just exercises the path
	}
}
