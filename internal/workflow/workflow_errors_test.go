package workflow

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"recac/internal/agent"
	"recac/internal/cmdutils"
	"recac/internal/docker"
	"recac/internal/git"
	"recac/internal/jira"
	"recac/internal/runner"

	"github.com/stretchr/testify/assert"
)

func TestProcessDirectTask_Errors(t *testing.T) {
	// Restore mocks after test
	originalSetup := cmdutils.SetupWorkspace
	defer func() { cmdutils.SetupWorkspace = originalSetup }()

	originalRunWorkflow := RunWorkflow
	defer func() { RunWorkflow = originalRunWorkflow }()

	// 1. SetupWorkspace Failure
	cmdutils.SetupWorkspace = func(ctx context.Context, gitClient git.IClient, repoURL, workspace, ticketID, epicKey, timestamp string) (string, error) {
		return "", errors.New("setup failed")
	}

	cfg := SessionConfig{
		RepoURL: "https://example.com/repo",
		Summary: "Task",
	}

	err := ProcessDirectTask(context.Background(), cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "setup failed")

	// 2. WriteFile Failure (Mock Setup Success, but Write Fail)
	cmdutils.SetupWorkspace = func(ctx context.Context, gitClient git.IClient, repoURL, workspace, ticketID, epicKey, timestamp string) (string, error) {
		return repoURL, nil
	}

	// Create a read-only directory or file to cause write error
	tmpDir, _ := os.MkdirTemp("", "workflow-fail")
	defer os.RemoveAll(tmpDir)

	// Create a directory where file should be
	os.Mkdir(fmt.Sprintf("%s/app_spec.txt", tmpDir), 0755)

	cfg.ProjectPath = tmpDir
	err = ProcessDirectTask(context.Background(), cfg)
	assert.Error(t, err)
	// WriteFile to a directory path fails with "is a directory"
	// assert.Contains(t, err.Error(), "app_spec.txt")

	// 3. RunWorkflow Failure
	// Fix ProjectPath to valid one
	tmpDir2, _ := os.MkdirTemp("", "workflow-run-fail")
	defer os.RemoveAll(tmpDir2)
	cfg.ProjectPath = tmpDir2

	RunWorkflow = func(ctx context.Context, cfg SessionConfig) error {
		return errors.New("run failed")
	}

	err = ProcessDirectTask(context.Background(), cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "run failed")
}

func TestProcessJiraTicket_Errors(t *testing.T) {
	// Restore mocks
	originalSetup := cmdutils.SetupWorkspace
	defer func() { cmdutils.SetupWorkspace = originalSetup }()

	originalRunWorkflow := RunWorkflow
	defer func() { RunWorkflow = originalRunWorkflow }()

	// Mock Server for Jira (returns error)
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	// 1. GetTicket Failure (500)
	mux.HandleFunc("/rest/api/3/issue/FAIL-1", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	jClient := jira.NewClient(server.URL, "user", "token")
	cfg := SessionConfig{IsMock: true}

	err := ProcessJiraTicket(context.Background(), "FAIL-1", jClient, cfg, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to fetch ticket")

	// 2. Invalid Ticket Format (200 but bad json or missing fields)
	mux.HandleFunc("/rest/api/3/issue/BAD-1", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{}`)) // Empty JSON, missing fields
	})

	err = ProcessJiraTicket(context.Background(), "BAD-1", jClient, cfg, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid ticket format")

	// 3. SetupWorkspace Failure (Mock Ticket Success, Setup Fail)
	mux.HandleFunc("/rest/api/3/issue/OK-1", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Valid ticket
		w.Write([]byte(`{
			"key": "OK-1",
			"fields": {
				"summary": "OK Ticket",
				"description": {
					"type": "doc",
					"version": 1,
					"content": [{"type":"paragraph", "content":[{"type":"text", "text":"Repo: https://example.com/repo"}]}]
				}
			}
		}`))
	})

	mux.HandleFunc("/rest/api/3/issue/OK-1/transitions", func(w http.ResponseWriter, r *http.Request) {
		// Accept transition silently or return empty transitions
		if r.Method == "POST" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.Write([]byte(`{"transitions":[]}`))
	})

	cmdutils.SetupWorkspace = func(ctx context.Context, gitClient git.IClient, repoURL, workspace, ticketID, epicKey, timestamp string) (string, error) {
		return "", errors.New("setup failed")
	}

	err = ProcessJiraTicket(context.Background(), "OK-1", jClient, cfg, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "setup failed")

	// 4. RunWorkflow Failure
	cmdutils.SetupWorkspace = func(ctx context.Context, gitClient git.IClient, repoURL, workspace, ticketID, epicKey, timestamp string) (string, error) {
		return repoURL, nil
	}
	RunWorkflow = func(ctx context.Context, cfg SessionConfig) error {
		return errors.New("run failed")
	}

	// Use temp dir for ProjectPath to avoid mkdir temp errors in ProcessJiraTicket
	tmpDir, _ := os.MkdirTemp("", "jira-run-fail")
	defer os.RemoveAll(tmpDir)
	cfg.ProjectPath = tmpDir

	err = ProcessJiraTicket(context.Background(), "OK-1", jClient, cfg, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "run failed")
}

func TestRunWorkflow_Errors(t *testing.T) {
	// Restore mocks
	originalGetAgent := cmdutils.GetAgentClient
	defer func() { cmdutils.GetAgentClient = originalGetAgent }()

	originalNewSession := NewSessionFunc
	defer func() { NewSessionFunc = originalNewSession }()

	// 1. GetAgentClient Failure
	cmdutils.GetAgentClient = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return nil, errors.New("agent init failed")
	}

	tmpDir, _ := os.MkdirTemp("", "run-fail")
	defer os.RemoveAll(tmpDir)

	cfg := SessionConfig{
		ProjectPath: tmpDir,
		SessionName: "fail-session",
		IsMock:      false,
		AllowDirty:  true,
	}

	err := RunWorkflow(context.Background(), cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "agent init failed")

	// 2. Session Start Failure
	cmdutils.GetAgentClient = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return agent.NewMockAgent(), nil
	}

	NewSessionFunc = func(d runner.DockerClient, a agent.Agent, workspace, image, project, provider, model string, maxAgents int) *runner.Session {
		// Return a session that fails Start
		// We can't easily mock Start method on Session struct without interface.
		// But Session.Start fails if Docker.RunContainer fails (if Docker is set).
		// We can inject a mock Docker client that fails.

		// Let's create a failing docker client
		failDocker := &failDockerClient{}

		s := runner.NewSession(failDocker, a, workspace, image, project, provider, model, maxAgents)
		return s
	}

	err = RunWorkflow(context.Background(), cfg)
	// Start fails because Docker RunContainer fails
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "docker run failed")
}

// failDockerClient implements runner.DockerClient and fails
type failDockerClient struct{}

func (f *failDockerClient) RunContainer(ctx context.Context, image string, workspace string, extraBinds []string, env []string, user string) (string, error) {
	return "", errors.New("docker run failed")
}
func (f *failDockerClient) CheckDaemon(ctx context.Context) error { return nil }
func (f *failDockerClient) ImageExists(ctx context.Context, image string) (bool, error) { return true, nil }
func (f *failDockerClient) PullImage(ctx context.Context, image string) error { return nil }
func (f *failDockerClient) ImageBuild(ctx context.Context, opts docker.ImageBuildOptions) (string, error) { return "id", nil }
func (f *failDockerClient) StopContainer(ctx context.Context, containerID string) error { return nil }
func (f *failDockerClient) Exec(ctx context.Context, containerID string, cmd []string) (string, error) { return "", nil }
func (f *failDockerClient) ExecAsUser(ctx context.Context, containerID string, user string, cmd []string) (string, error) { return "", nil }
