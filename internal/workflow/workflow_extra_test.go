package workflow

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"recac/internal/jira"
	"recac/internal/runner"

	"github.com/stretchr/testify/assert"
)

// SimpleMockSessionManager implements ISessionManager
type SimpleMockSessionManager struct {
	StartSessionFunc func(name, goal string, command []string, cwd string) (*runner.SessionState, error)
}

func (m *SimpleMockSessionManager) StartSession(name, goal string, command []string, cwd string) (*runner.SessionState, error) {
	if m.StartSessionFunc != nil {
		return m.StartSessionFunc(name, goal, command, cwd)
	}
	return &runner.SessionState{PID: 12345, LogFile: "/tmp/mock.log"}, nil
}

func TestRunWorkflow_Detached_Check(t *testing.T) {
	// Setup mock SessionManager
	mockSM := &SimpleMockSessionManager{
		StartSessionFunc: func(name, goal string, command []string, cwd string) (*runner.SessionState, error) {
			assert.Equal(t, "DETACHED-SESSION", name)
			return &runner.SessionState{PID: 999, LogFile: "/tmp/detached.log"}, nil
		},
	}

	cfg := SessionConfig{
		SessionName:    "DETACHED-SESSION",
		Detached:       true,
		SessionManager: mockSM,
		ProjectPath:    "/tmp/test-project",
	}

	// We need to ensure os.Executable check passes or fallback works.
	// Since we can't easily mock os.Executable, we rely on the fact that "go test" creates an executable.

	err := RunWorkflow(context.Background(), cfg)
	assert.NoError(t, err)
}

func TestProcessJiraTicket_ExtraErrorPaths(t *testing.T) {
	// Mock Jira Server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Mock GetTicket
		if r.URL.Path == "/rest/api/3/issue/INVALID-1" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	jClient := jira.NewClient(server.URL, "user", "token")

	// Test 1: Invalid Ticket
	err := ProcessJiraTicket(context.Background(), "INVALID-1", jClient, SessionConfig{}, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to fetch ticket")
}

func TestProcessDirectTask_WorkspaceFail(t *testing.T) {
	// Test failure to setup workspace (e.g. invalid repo URL that causes git failure)

	cfg := SessionConfig{
		RepoURL: "/non/existent/repo",
		ProjectPath: "/tmp/test-project-direct-fail",
	}

	err := ProcessDirectTask(context.Background(), cfg)
	assert.Error(t, err)
}

func TestProcessJiraTicket_WorkspaceCreationFail(t *testing.T) {
	// Mock Jira Server for a valid ticket
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"key": "VALID-1",
			"fields": {
				"summary": "Valid Ticket",
				"description": {
					"type": "doc",
					"content": [{"type": "paragraph", "content": [{"type": "text", "text": "Repo: https://github.com/example/repo"}]}]
				}
			}
		}`))
	}))
	defer server.Close()

	jClient := jira.NewClient(server.URL, "user", "token")

	// Provide a ProjectPath that is a file, so MkdirAll fails
	tmpFile := filepath.Join(os.TempDir(), "recac-test-file")
	os.WriteFile(tmpFile, []byte("content"), 0644)
	defer os.Remove(tmpFile)

	cfg := SessionConfig{
		ProjectPath: tmpFile, // This should cause MkdirAll to fail
	}

	err := ProcessJiraTicket(context.Background(), "VALID-1", jClient, cfg, nil)
	assert.Error(t, err)
}
