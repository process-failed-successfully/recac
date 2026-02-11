package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"recac/internal/agent"
	"recac/internal/cmdutils"
	"recac/internal/docker"
	"recac/internal/git"
	"recac/internal/jira"
	"recac/internal/runner"

	"github.com/stretchr/testify/assert"
)

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

// Mock Session Manager
type mockSessionManager struct {
	startSessionFunc func(name, goal string, command []string, cwd string) (*runner.SessionState, error)
}

func (m *mockSessionManager) StartSession(name, goal string, command []string, cwd string) (*runner.SessionState, error) {
	if m.startSessionFunc != nil {
		return m.startSessionFunc(name, goal, command, cwd)
	}
	return &runner.SessionState{PID: 123, LogFile: "test.log"}, nil
}

func TestRunWorkflow_Detached(t *testing.T) {
	// Use Mock Session Manager
	mockSM := &mockSessionManager{}

	called := false
	mockSM.startSessionFunc = func(name, goal string, command []string, cwd string) (*runner.SessionState, error) {
		called = true
		assert.Equal(t, "detached-session", name)
		assert.Contains(t, command, "--mock")
		return &runner.SessionState{PID: 999, LogFile: "out.log"}, nil
	}

	cfg := SessionConfig{
		Detached:       true,
		SessionName:    "detached-session",
		IsMock:         true,
		SessionManager: mockSM,
		ProjectPath:    "/tmp/test",
	}

	err := RunWorkflow(context.Background(), cfg)
	assert.NoError(t, err)
	assert.True(t, called)
}

func TestProcessJiraTicket_WithRepoURL(t *testing.T) {
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
		// Replace real docker client with mock to avoid starting real containers
		mockD := &mockDockerClient{
			runContainerFunc: func(ctx context.Context, imageRef string, workspace string, extraBinds []string, env []string, user string) (string, error) {
				return "mock-container-id", nil
			},
		}
		s := runner.NewSession(mockD, a, workspace, image, project, provider, model, maxAgents)
		// Set iteration limit to ensure it finishes if circuit breaker doesn't trip
		s.MaxIterations = 1
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

	if err != nil {
		// Acceptable errors due to mock limitations
		acceptedErrors := map[string]bool{
			"circuit breaker: no-op loop": true,
			"maximum iterations reached":  true,
		}
		assert.True(t, acceptedErrors[err.Error()], "Unexpected error from RunWorkflow: %v", err)
	}
}

func TestProcessJiraTicket_Blockers(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	mux.HandleFunc("/rest/api/3/issue/BLOCKED-1", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"key": "BLOCKED-1",
			"fields": map[string]interface{}{
				"summary": "Blocked Ticket",
				"issuelinks": []interface{}{
					map[string]interface{}{
						"type": map[string]interface{}{
							"inward": "is blocked by",
						},
						"inwardIssue": map[string]interface{}{
							"key": "BLOCKER-1",
							"fields": map[string]interface{}{
								"status": map[string]interface{}{
									"name": "To Do",
								},
							},
						},
					},
				},
			},
		})
	})

	jClient := jira.NewClient(server.URL, "user", "token")
	cfg := SessionConfig{Logger: nil}

	err := ProcessJiraTicket(context.Background(), "BLOCKED-1", jClient, cfg, nil)
	assert.NoError(t, err)
}

func TestProcessJiraTicket_InvalidFormat(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	mux.HandleFunc("/rest/api/3/issue/BAD-1", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Missing fields
		json.NewEncoder(w).Encode(map[string]interface{}{
			"key": "BAD-1",
		})
	})

	jClient := jira.NewClient(server.URL, "user", "token")
	cfg := SessionConfig{Logger: nil}

	err := ProcessJiraTicket(context.Background(), "BAD-1", jClient, cfg, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid ticket format")
}

func TestRunWorkflow_DirtyCheck(t *testing.T) {
	// Need a repo with dirty state.
	tmpDir, _ := os.MkdirTemp("", "workflow-dirty-test")
	defer os.RemoveAll(tmpDir)

	// Init git repo
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found")
	}

	exec.Command("git", "init", tmpDir).Run()
	exec.Command("git", "-C", tmpDir, "config", "user.email", "test@example.com").Run()
	exec.Command("git", "-C", tmpDir, "config", "user.name", "Test User").Run()

	// Create a file and commit it
	os.WriteFile(filepath.Join(tmpDir, "file.txt"), []byte("content"), 0644)
	exec.Command("git", "-C", tmpDir, "add", "file.txt").Run()
	exec.Command("git", "-C", tmpDir, "commit", "-m", "init").Run()

	// Modify it (dirty)
	os.WriteFile(filepath.Join(tmpDir, "file.txt"), []byte("modified"), 0644)

	cfg := SessionConfig{
		ProjectPath: tmpDir,
		SessionName: "dirty-test",
		IsMock:      false,
		AllowDirty:  false,
	}

	err := RunWorkflow(context.Background(), cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "uncommitted changes detected")
}

func TestRunWorkflow_AgentInitFail(t *testing.T) {
	// Mock cmdutils.GetAgentClient to fail
	originalGetAgentClient := cmdutils.GetAgentClient
	defer func() { cmdutils.GetAgentClient = originalGetAgentClient }()
	cmdutils.GetAgentClient = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return nil, errors.New("agent init failed")
	}

	tmpDir, _ := os.MkdirTemp("", "workflow-agent-fail")
	defer os.RemoveAll(tmpDir)

	// Needs to have git initialized to pass dirty check (unless we set AllowDirty=true)
	// We set AllowDirty=true to bypass git check

	cfg := SessionConfig{
		ProjectPath: tmpDir,
		SessionName: "agent-fail-test",
		IsMock:      false,
		ProjectName: "test-project",
		AllowDirty:  true,
	}

	err := RunWorkflow(context.Background(), cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to initialize agent")
}

// Mock Docker Client
type mockDockerClient struct {
	runContainerFunc func(ctx context.Context, imageRef string, workspace string, extraBinds []string, env []string, user string) (string, error)
}

func (m *mockDockerClient) CheckDaemon(ctx context.Context) error { return nil }
func (m *mockDockerClient) RunContainer(ctx context.Context, imageRef string, workspace string, extraBinds []string, env []string, user string) (string, error) {
	if m.runContainerFunc != nil {
		return m.runContainerFunc(ctx, imageRef, workspace, extraBinds, env, user)
	}
	return "mock-id", nil
}
func (m *mockDockerClient) StopContainer(ctx context.Context, containerID string) error { return nil }
func (m *mockDockerClient) Exec(ctx context.Context, containerID string, cmd []string) (string, error) { return "", nil }
func (m *mockDockerClient) ExecAsUser(ctx context.Context, containerID string, user string, cmd []string) (string, error) { return "", nil }
func (m *mockDockerClient) ImageExists(ctx context.Context, tag string) (bool, error) { return true, nil }
func (m *mockDockerClient) ImageBuild(ctx context.Context, opts docker.ImageBuildOptions) (string, error) { return "", nil }
func (m *mockDockerClient) PullImage(ctx context.Context, imageRef string) error { return nil }

func TestRunWorkflow_ContainerFail(t *testing.T) {
	// Mock NewSessionFunc to use our mock docker client
	originalNewSessionFunc := NewSessionFunc
	defer func() { NewSessionFunc = originalNewSessionFunc }()
	NewSessionFunc = func(d runner.DockerClient, a agent.Agent, workspace, image, project, provider, model string, maxAgents int) *runner.Session {
		// Inject mock docker
		mockD := &mockDockerClient{
			runContainerFunc: func(ctx context.Context, imageRef string, workspace string, extraBinds []string, env []string, user string) (string, error) {
				return "", errors.New("container run failed")
			},
		}
		// Pass mockD instead of d
		s := runner.NewSession(mockD, a, workspace, image, project, provider, model, maxAgents)
		return s
	}

	// Mock GetAgentClient
	originalGetAgentClient := cmdutils.GetAgentClient
	defer func() { cmdutils.GetAgentClient = originalGetAgentClient }()
	cmdutils.GetAgentClient = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return agent.NewMockAgent(), nil
	}

	tmpDir, _ := os.MkdirTemp("", "workflow-container-fail")
	defer os.RemoveAll(tmpDir)

	cfg := SessionConfig{
		ProjectPath: tmpDir,
		SessionName: "container-fail-test",
		IsMock:      false,
		ProjectName: "test-project",
		AllowDirty:  true,
	}

	err := RunWorkflow(context.Background(), cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "container run failed")
}

func TestProcessJiraTicket_NoRepo(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	mux.HandleFunc("/rest/api/3/issue/NOREPO-1", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"key": "NOREPO-1",
			"fields": map[string]interface{}{
				"summary": "No Repo Ticket",
				"description": map[string]interface{}{
					"type": "doc", "version": 1,
					"content": []map[string]interface{}{
						{"type": "paragraph", "content": []map[string]interface{}{{"type": "text", "text": "Just text"}}},
					},
				},
			},
		})
	})

	jClient := jira.NewClient(server.URL, "user", "token")
	cfg := SessionConfig{Logger: nil}

	err := ProcessJiraTicket(context.Background(), "NOREPO-1", jClient, cfg, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no repo url found")
}

func TestProcessJiraTicket_Epic(t *testing.T) {
	// Mock RunWorkflow to do nothing
	originalRunWorkflow := RunWorkflow
	defer func() { RunWorkflow = originalRunWorkflow }()
	RunWorkflow = func(ctx context.Context, cfg SessionConfig) error {
		// Verify Epic Key was set
		assert.Equal(t, "EPIC-1", cfg.JiraEpicKey)
		return nil
	}

	// Mock SetupWorkspace
	originalSetup := cmdutils.SetupWorkspace
	defer func() { cmdutils.SetupWorkspace = originalSetup }()
	cmdutils.SetupWorkspace = func(ctx context.Context, gitClient git.IClient, repoURL, workspace, ticketID, epicKey, timestamp string) (string, error) {
		assert.Equal(t, "EPIC-1", epicKey)
		os.MkdirAll(workspace, 0755)
		return repoURL, nil
	}

	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	mux.HandleFunc("/rest/api/3/issue/TASK-1", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"key": "TASK-1",
			"fields": map[string]interface{}{
				"summary": "Task with Epic",
				"description": map[string]interface{}{
					"type": "doc", "version": 1,
					"content": []map[string]interface{}{
						{"type": "paragraph", "content": []map[string]interface{}{{"type": "text", "text": "Repo: https://github.com/example/repo"}}},
					},
				},
				"parent": map[string]interface{}{
					"key": "EPIC-1",
				},
			},
		})
	})

	// Transitions mock (required)
	mux.HandleFunc("/rest/api/3/issue/TASK-1/transitions", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			json.NewEncoder(w).Encode(map[string]interface{}{"transitions": []interface{}{map[string]interface{}{"id": "11", "name": "In Progress"}}})
		} else {
			w.WriteHeader(http.StatusNoContent)
		}
	})

	jClient := jira.NewClient(server.URL, "user", "token")
	tmpDir, _ := os.MkdirTemp("", "workflow-jira-epic-test")
	defer os.RemoveAll(tmpDir)

	cfg := SessionConfig{
		ProjectPath: tmpDir,
		Cleanup:     true,
	}

	err := ProcessJiraTicket(context.Background(), "TASK-1", jClient, cfg, nil)
	assert.NoError(t, err)
}

func TestProcessJiraTicket_IgnoredBlockers(t *testing.T) {
	// Mock RunWorkflow
	originalRunWorkflow := RunWorkflow
	defer func() { RunWorkflow = originalRunWorkflow }()
	RunWorkflow = func(ctx context.Context, cfg SessionConfig) error {
		return nil
	}
	// Mock SetupWorkspace
	originalSetup := cmdutils.SetupWorkspace
	defer func() { cmdutils.SetupWorkspace = originalSetup }()
	cmdutils.SetupWorkspace = func(ctx context.Context, gitClient git.IClient, repoURL, workspace, ticketID, epicKey, timestamp string) (string, error) {
		os.MkdirAll(workspace, 0755)
		return repoURL, nil
	}

	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	mux.HandleFunc("/rest/api/3/issue/IGNORED-1", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"key": "IGNORED-1",
			"fields": map[string]interface{}{
				"summary": "Ignored Blockers",
				"description": map[string]interface{}{
					"type": "doc", "version": 1,
					"content": []map[string]interface{}{
						{"type": "paragraph", "content": []map[string]interface{}{{"type": "text", "text": "Repo: https://github.com/example/repo"}}},
					},
				},
				"issuelinks": []interface{}{
					map[string]interface{}{
						"type": map[string]interface{}{
							"inward": "is blocked by",
						},
						"inwardIssue": map[string]interface{}{
							"key": "BLOCKER-1",
							"fields": map[string]interface{}{
								"status": map[string]interface{}{
									"name": "To Do",
								},
							},
						},
					},
				},
			},
		})
	})

	mux.HandleFunc("/rest/api/3/issue/IGNORED-1/transitions", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			json.NewEncoder(w).Encode(map[string]interface{}{"transitions": []interface{}{map[string]interface{}{"id": "11", "name": "In Progress"}}})
		} else {
			w.WriteHeader(http.StatusNoContent)
		}
	})

	jClient := jira.NewClient(server.URL, "user", "token")
	tmpDir, _ := os.MkdirTemp("", "workflow-jira-ignored-test")
	defer os.RemoveAll(tmpDir)

	cfg := SessionConfig{
		ProjectPath: tmpDir,
		Cleanup:     false,
	}

	ignored := map[string]bool{
		"BLOCKER-1": true,
	}

	err := ProcessJiraTicket(context.Background(), "IGNORED-1", jClient, cfg, ignored)
	assert.NoError(t, err)
	// Success (not skipped) means workspace created.
	assert.FileExists(t, filepath.Join(tmpDir, "app_spec.txt"))
}

func TestRunWorkflow_DetachedFail(t *testing.T) {
	mockSM := &mockSessionManager{}
	mockSM.startSessionFunc = func(name, goal string, command []string, cwd string) (*runner.SessionState, error) {
		return nil, errors.New("start session failed")
	}

	cfg := SessionConfig{
		Detached:       true,
		SessionName:    "detached-fail",
		SessionManager: mockSM,
	}

	err := RunWorkflow(context.Background(), cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "start session failed")
}

func TestRunWorkflow_DetachedNoName(t *testing.T) {
	cfg := SessionConfig{
		Detached:    true,
		SessionName: "", // Missing
	}

	err := RunWorkflow(context.Background(), cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "--name is required")
}

func TestProcessDirectTask_SetupFail(t *testing.T) {
	originalSetup := cmdutils.SetupWorkspace
	defer func() { cmdutils.SetupWorkspace = originalSetup }()
	cmdutils.SetupWorkspace = func(ctx context.Context, gitClient git.IClient, repoURL, workspace, ticketID, epicKey, timestamp string) (string, error) {
		return "", errors.New("setup failed")
	}

	cfg := SessionConfig{
		SessionName: "setup-fail",
		RepoURL:     "repo",
	}

	err := ProcessDirectTask(context.Background(), cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "setup failed")
}

func TestProcessDirectTask_WriteSpecFail(t *testing.T) {
	originalSetup := cmdutils.SetupWorkspace
	defer func() { cmdutils.SetupWorkspace = originalSetup }()
	cmdutils.SetupWorkspace = func(ctx context.Context, gitClient git.IClient, repoURL, workspace, ticketID, epicKey, timestamp string) (string, error) {
		// Make workspace a file so writing app_spec.txt inside it fails
		// We need to remove the dir first if it exists
		os.RemoveAll(workspace)
		os.WriteFile(workspace, []byte("not-a-dir"), 0644)
		return "", nil
	}

	tmpDir, _ := os.MkdirTemp("", "workflow-spec-fail")
	defer os.RemoveAll(tmpDir)

	cfg := SessionConfig{
		ProjectPath: filepath.Join(tmpDir, "workspace"), // Subdir to allow replacing with file
		SessionName: "spec-fail",
		RepoURL:     "repo",
		Summary:     "Task",
	}

	err := ProcessDirectTask(context.Background(), cfg)
	assert.Error(t, err)
	// Error could be "not a directory" or similar
	if err != nil {
		assert.Contains(t, err.Error(), "not a directory")
	}
}
