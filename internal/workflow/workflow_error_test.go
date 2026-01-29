package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"recac/internal/agent"
	"recac/internal/cmdutils"
	"recac/internal/git"
	"recac/internal/jira"

	"github.com/stretchr/testify/assert"
)

func TestProcessDirectTask_Errors(t *testing.T) {
	// Common mocks restoration
	originalSetup := cmdutils.SetupWorkspace
	defer func() { cmdutils.SetupWorkspace = originalSetup }()
	originalRunWorkflow := RunWorkflow
	defer func() { RunWorkflow = originalRunWorkflow }()

	tests := []struct {
		name          string
		setupFunc     func() (string, error)
		runFunc       func() error
		projectPath   string
		expectError   string
	}{
		{
			name: "SetupWorkspace Failure",
			setupFunc: func() (string, error) {
				return "", errors.New("setup failed")
			},
			runFunc:     func() error { return nil },
			projectPath: "/tmp/test",
			expectError: "setup failed",
		},
		{
			name: "Write App Spec Failure",
			setupFunc: func() (string, error) {
				return "https://github.com/test/repo", nil
			},
			runFunc: func() error { return nil },
			// projectPath will be set to a file path in the test loop
			expectError: "not a directory",
		},
		{
			name: "RunWorkflow Failure",
			setupFunc: func() (string, error) {
				return "https://github.com/test/repo", nil
			},
			runFunc: func() error {
				return errors.New("workflow run failed")
			},
			projectPath: "/tmp/test", // Will be replaced by temp dir in loop
			expectError: "workflow run failed",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Mock SetupWorkspace
			cmdutils.SetupWorkspace = func(ctx context.Context, gitClient git.IClient, repoURL, workspace, ticketID, epicKey, timestamp string) (string, error) {
				return tc.setupFunc()
			}

			// Mock RunWorkflow
			RunWorkflow = func(ctx context.Context, cfg SessionConfig) error {
				return tc.runFunc()
			}

			// Prepare paths
			var path string
			if tc.name == "Write App Spec Failure" {
				tmpFile, err := os.CreateTemp("", "workflow-test-file")
				assert.NoError(t, err)
				defer os.Remove(tmpFile.Name())
				tmpFile.Close()
				path = tmpFile.Name()
			} else if tc.name == "RunWorkflow Failure" {
				tmpDir, err := os.MkdirTemp("", "workflow-run-test")
				assert.NoError(t, err)
				defer os.RemoveAll(tmpDir)
				path = tmpDir
			} else {
				path = tc.projectPath
			}

			cfg := SessionConfig{
				ProjectPath: path,
				RepoURL:     "https://github.com/test/repo",
				SessionName: "test-session",
				Summary:     "Summary", // Needed for Write App Spec test
			}

			err := ProcessDirectTask(context.Background(), cfg)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tc.expectError)
		})
	}
}

func TestProcessJiraTicket_Errors(t *testing.T) {
	tests := []struct {
		name          string
		handler       func(w http.ResponseWriter, r *http.Request)
		projectPath   string // if empty, uses temp dir
		setupPath     func() string
		cleanupPath   func(string)
		expectError   string
	}{
		{
			name: "GetTicket Failure",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			expectError: "failed to fetch ticket",
		},
		{
			name: "Mkdir Failure",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/rest/api/3/issue/TEST-1" {
					json.NewEncoder(w).Encode(map[string]interface{}{
						"key": "TEST-1",
						"fields": map[string]interface{}{
							"summary": "Test Ticket",
							"description": map[string]interface{}{},
						},
					})
					return
				}
				if r.URL.Path == "/rest/api/3/issue/TEST-1/transitions" {
					w.WriteHeader(http.StatusOK)
					return
				}
				w.WriteHeader(http.StatusNotFound)
			},
			setupPath: func() string {
				tmpFile, _ := os.CreateTemp("", "workflow-mkdir-test")
				tmpFile.Close()
				return tmpFile.Name()
			},
			cleanupPath: func(p string) {
				os.Remove(p)
			},
			expectError: "not a directory",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(tc.handler))
			defer server.Close()

			client := jira.NewClient(server.URL, "user", "token")

			path := tc.projectPath
			if tc.setupPath != nil {
				path = tc.setupPath()
				defer tc.cleanupPath(path)
			}

			cfg := SessionConfig{
				SessionName: "test-jira-error",
				ProjectPath: path,
			}

			err := ProcessJiraTicket(context.Background(), "TEST-1", client, cfg, nil)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tc.expectError)
		})
	}
}

func TestRunWorkflow_Errors(t *testing.T) {
	originalGetAgent := cmdutils.GetAgentClient
	defer func() { cmdutils.GetAgentClient = originalGetAgent }()

	tests := []struct {
		name        string
		getAgentErr error
		expectError string
	}{
		{
			name:        "GetAgentClient Failure",
			getAgentErr: errors.New("agent init failed"),
			expectError: "failed to initialize agent",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cmdutils.GetAgentClient = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
				return nil, tc.getAgentErr
			}

			tmpDir, err := os.MkdirTemp("", "workflow-agent-test")
			assert.NoError(t, err)
			defer os.RemoveAll(tmpDir)

			cfg := SessionConfig{
				ProjectPath: tmpDir,
				SessionName: "test-agent-err",
				AllowDirty:  true,
			}

			err = RunWorkflow(context.Background(), cfg)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tc.expectError)
		})
	}
}
