package workflow

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"errors"
	"recac/internal/agent"
	"recac/internal/docker"
	"recac/internal/jira"
	"recac/internal/runner"
)

type BadMockDocker struct{}

func (m *BadMockDocker) CheckDaemon(ctx context.Context) error { return nil }
func (m *BadMockDocker) RunContainer(ctx context.Context, imageRef string, workspace string, extraBinds []string, env []string, user string) (string, error) {
	return "", errors.New("container start failed")
}
func (m *BadMockDocker) StopContainer(ctx context.Context, containerID string) error { return nil }
func (m *BadMockDocker) Exec(ctx context.Context, containerID string, cmd []string) (string, error) {
	return "", nil
}
func (m *BadMockDocker) ExecAsUser(ctx context.Context, containerID string, user string, cmd []string) (string, error) {
	return "", nil
}
func (m *BadMockDocker) ImageExists(ctx context.Context, tag string) (bool, error) { return true, nil }
func (m *BadMockDocker) ImageBuild(ctx context.Context, opts docker.ImageBuildOptions) (string, error) {
	return "", nil
}
func (m *BadMockDocker) PullImage(ctx context.Context, imageRef string) error { return nil }

func TestRunWorkflow_MockStartError(t *testing.T) {
	// Backup and Restore NewSessionFunc
	originalNewSessionFunc := NewSessionFunc
	defer func() { NewSessionFunc = originalNewSessionFunc }()

	// Inject Bad Session
	NewSessionFunc = func(d runner.DockerClient, a agent.Agent, workspace, image, project, provider, model string, maxAgents int) *runner.Session {
		// Ignore 'd' and use BadMockDocker
		// We still need to call the real NewSession to get a valid struct, but with our bad docker client
		// But wait, NewSession does initialization logic (DB, etc).
		// We can just create the struct manually if we want to avoid DB init?
		// But RunWorkflow calls NewSessionFunc.
		// If we use runner.NewSession, it tries to init DB.
		// We should use NewSessionWithConfig or similar if possible, or mock DB.
		// Or we can just let it fail DB and see.
		// Actually, `NewSession` handles DB retry and exit(1) on failure! We must avoid that in test.
		// We should construct *runner.Session manually or use a constructor that doesn't exit.
		// runner.NewSessionWithConfig does not exit.

		sess := runner.NewSessionWithConfig(workspace, project, provider, model, nil)
		sess.Docker = &BadMockDocker{}
		sess.Agent = a
		sess.Image = image
		// We need to set other fields that RunWorkflow expects?
		return sess
	}

	cfg := SessionConfig{
		IsMock:      true,
		SessionName: "bad-start-session",
	}

	err := RunWorkflow(context.Background(), cfg)
	if err == nil {
		t.Error("Expected error from Session.Start")
	}
	if !strings.Contains(err.Error(), "container start failed") {
		t.Errorf("Expected 'container start failed', got %v", err)
	}
}

func TestProcessJiraTicket_GetTicketError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := jira.NewClient(server.URL, "user", "token")
	cfg := SessionConfig{}

	err := ProcessJiraTicket(context.Background(), "PROJ-1", client, cfg, nil)
	if err == nil {
		t.Error("Expected error from GetTicket")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("Expected 500 error, got %v", err)
	}
}

func TestProcessJiraTicket_Blocked(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/rest/api/3/issue/PROJ-1" {
			w.WriteHeader(http.StatusOK)
			// Return ticket with blockers
			w.Write([]byte(`{
				"fields": {
					"issuelinks": [
						{
							"type": {"inward": "is blocked by"},
							"inwardIssue": {
								"key": "BLOCK-1",
								"fields": {"status": {"name": "In Progress"}}
							}
						}
					]
				}
			}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := jira.NewClient(server.URL, "user", "token")
	cfg := SessionConfig{}

	// Should return nil (skipped)
	err := ProcessJiraTicket(context.Background(), "PROJ-1", client, cfg, map[string]bool{})
	if err != nil {
		t.Errorf("Expected nil error (skip), got %v", err)
	}
}

func TestProcessJiraTicket_MissingFields(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/rest/api/3/issue/PROJ-1" {
			w.WriteHeader(http.StatusOK)
			// Return ticket with no fields
			w.Write([]byte(`{}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := jira.NewClient(server.URL, "user", "token")
	cfg := SessionConfig{}

	err := ProcessJiraTicket(context.Background(), "PROJ-1", client, cfg, nil)
	if err == nil {
		t.Error("Expected error due to missing fields")
	}
	if err.Error() != "invalid ticket format" {
		t.Errorf("Expected 'invalid ticket format', got %v", err)
	}
}

func TestProcessJiraTicket_NoRepoURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/rest/api/3/issue/PROJ-1" {
			w.WriteHeader(http.StatusOK)
			// Return valid ticket but no Repo URL in description
			w.Write([]byte(`{
				"fields": {
					"summary": "Task",
					"description": {
						"type": "doc",
						"content": [{"type": "paragraph", "content": [{"type": "text", "text": "No repo here"}]}]
					}
				}
			}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := jira.NewClient(server.URL, "user", "token")
	cfg := SessionConfig{}

	err := ProcessJiraTicket(context.Background(), "PROJ-1", client, cfg, nil)
	if err == nil {
		t.Error("Expected error due to missing Repo URL")
	}
	if err.Error() != "no repo url found" {
		t.Errorf("Expected 'no repo url found', got %v", err)
	}
}

func TestRunWorkflow_AgentClientError(t *testing.T) {
	// Normal mode (not mock, not detached)
	cfg := SessionConfig{
		ProjectPath: ".",
		Provider:    "invalid-provider", // Should cause GetAgentClient to fail
	}

	// We need to bypass the "uncommitted changes" check if running locally in a dirty repo
	cfg.AllowDirty = true

	err := RunWorkflow(context.Background(), cfg)
	if err == nil {
		t.Error("Expected error from GetAgentClient")
	}
	if !strings.Contains(err.Error(), "failed to initialize agent") {
		t.Errorf("Expected 'failed to initialize agent', got %v", err)
	}
}
