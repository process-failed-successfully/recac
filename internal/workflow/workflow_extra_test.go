package workflow

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"recac/internal/cmdutils"
	"recac/internal/git"
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

	err := RunWorkflow(context.Background(), cfg)
	assert.NoError(t, err)
}

func TestProcessJiraTicket_ExtraErrorPaths(t *testing.T) {
	// Mock Jira Server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	cfg := SessionConfig{
		RepoURL:     "/non/existent/repo",
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

func TestWorkflowTableDriven(t *testing.T) {
	// Start Jira mock server
	jiraMux := http.NewServeMux()
	jiraMux.HandleFunc("/rest/api/3/issue/VALID-1", func(w http.ResponseWriter, r *http.Request) {
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
	})
	jiraMux.HandleFunc("/rest/api/3/issue/VALID-NO-REPO", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"key": "VALID-NO-REPO",
			"fields": {
				"summary": "Valid Ticket",
				"description": {
					"type": "doc",
					"content": [{"type": "paragraph", "content": [{"type": "text", "text": "No repo url here"}]}]
				}
			}
		}`))
	})
	server := httptest.NewServer(jiraMux)
	defer server.Close()

	jClient := jira.NewClient(server.URL, "user", "token")

	tests := []struct {
		name          string
		testType      string // "direct", "jira", "run"
		setupMock     func() func()
		cfg           SessionConfig
		jiraTicketID  string
		expectedError string
	}{
		{
			name:     "ProcessDirectTask MkdirTemp Fail",
			testType: "direct",
			setupMock: func() func() {
				// Use t.Setenv for standard manipulation
				return nil
			},
			cfg: SessionConfig{
				ProjectPath: "", // Force MkdirTemp
				RepoURL:     "https://github.com/example/repo",
			},
			expectedError: "not a directory",
		},
		{
			name:     "ProcessDirectTask Write Spec Fail",
			testType: "direct",
			setupMock: func() func() {
				originalSetup := cmdutils.SetupWorkspace
				cmdutils.SetupWorkspace = func(ctx context.Context, gitClient git.IClient, repoURL, workspace, ticketID, epicKey, timestamp string) (string, error) {
					return repoURL, nil
				}
				return func() { cmdutils.SetupWorkspace = originalSetup }
			},
			cfg: SessionConfig{
				RepoURL: "https://github.com/example/repo",
				Summary: "Test Write Fail",
				// ProjectPath will be set in the test runner
			},
			expectedError: "permission denied",
		},
		{
			name:     "ProcessDirectTask RunWorkflow Fail",
			testType: "direct",
			setupMock: func() func() {
				originalSetup := cmdutils.SetupWorkspace
				cmdutils.SetupWorkspace = func(ctx context.Context, gitClient git.IClient, repoURL, workspace, ticketID, epicKey, timestamp string) (string, error) {
					return repoURL, nil
				}
				originalRunWorkflow := RunWorkflow
				RunWorkflow = func(ctx context.Context, cfg SessionConfig) error {
					return fmt.Errorf("mock run workflow error")
				}
				return func() {
					cmdutils.SetupWorkspace = originalSetup
					RunWorkflow = originalRunWorkflow
				}
			},
			cfg: SessionConfig{
				RepoURL: "https://github.com/example/repo",
				Summary: "Test RunWorkflow Fail",
				// ProjectPath will be set in the test runner
			},
			expectedError: "mock run workflow error",
		},
		{
			name:     "ProcessJiraTicket MkdirTemp Fail",
			testType: "jira",
			setupMock: func() func() {
				return nil
			},
			cfg: SessionConfig{
				ProjectPath: "", // Trigger MkdirTemp
			},
			jiraTicketID:  "VALID-1",
			expectedError: "not a directory",
		},
		{
			name:     "ProcessJiraTicket No Repo URL",
			testType: "jira",
			setupMock: func() func() {
				return nil
			},
			cfg: SessionConfig{
				ProjectPath: "/tmp/test-project-norepo",
			},
			jiraTicketID:  "VALID-NO-REPO",
			expectedError: "no repo url found",
		},
		{
			name:     "ProcessJiraTicket Write Spec Fail",
			testType: "jira",
			setupMock: func() func() {
				originalSetup := cmdutils.SetupWorkspace
				cmdutils.SetupWorkspace = func(ctx context.Context, gitClient git.IClient, repoURL, workspace, ticketID, epicKey, timestamp string) (string, error) {
					return repoURL, nil
				}
				return func() { cmdutils.SetupWorkspace = originalSetup }
			},
			cfg:           SessionConfig{},
			jiraTicketID:  "VALID-1",
			expectedError: "permission denied",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setupMock != nil {
				cleanup := tt.setupMock()
				if cleanup != nil {
					defer cleanup()
				}
			}

			// Handle path specific setup
			if tt.name == "ProcessDirectTask MkdirTemp Fail" || tt.name == "ProcessJiraTicket MkdirTemp Fail" {
				tmpFile, _ := os.CreateTemp("", "bad-tmp-*")
				defer os.Remove(tmpFile.Name())
				t.Setenv("TMPDIR", tmpFile.Name())
			} else if tt.name == "ProcessDirectTask Write Spec Fail" || tt.name == "ProcessDirectTask RunWorkflow Fail" || tt.name == "ProcessJiraTicket Write Spec Fail" {
				tmpDir, _ := os.MkdirTemp("", "write-fail-*")
				defer os.RemoveAll(tmpDir)

				if tt.name != "ProcessDirectTask RunWorkflow Fail" {
					os.Chmod(tmpDir, 0555) // Read-only
				}
				tt.cfg.ProjectPath = tmpDir
			}

			var err error
			if tt.testType == "direct" {
				err = ProcessDirectTask(context.Background(), tt.cfg)
			} else if tt.testType == "jira" {
				err = ProcessJiraTicket(context.Background(), tt.jiraTicketID, jClient, tt.cfg, nil)
			} else if tt.testType == "run" {
				err = RunWorkflow(context.Background(), tt.cfg)
			}

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
