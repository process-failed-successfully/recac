package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"recac/internal/cmdutils"
	"recac/internal/git"
	"recac/internal/jira"
	"recac/internal/telemetry"

	"github.com/stretchr/testify/assert"
)

func TestProcessDirectTask_WorkspaceError(t *testing.T) {
	// Mock SetupWorkspace to fail
	originalSetup := cmdutils.SetupWorkspace
	defer func() { cmdutils.SetupWorkspace = originalSetup }()

	cmdutils.SetupWorkspace = func(ctx context.Context, gitClient git.IClient, repoURL, workspace, ticketID, epicKey, timestamp string) (string, error) {
		return "", errors.New("setup failed")
	}

	cfg := SessionConfig{
		RepoURL: "https://github.com/example/repo",
		Logger:  telemetry.NewLogger(true, "", false),
	}

	err := ProcessDirectTask(context.Background(), cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "setup failed")
}

func TestProcessJiraTicket_Blocked(t *testing.T) {
	// Mock Jira Server
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	mux.HandleFunc("/rest/api/3/issue/BLOCKED-1", func(w http.ResponseWriter, r *http.Request) {
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
									"name": "In Progress",
								},
							},
						},
					},
				},
			},
		})
	})

	jClient := jira.NewClient(server.URL, "user", "token")

	// Track if SetupWorkspace is called
	setupCalled := false
	originalSetup := cmdutils.SetupWorkspace
	defer func() { cmdutils.SetupWorkspace = originalSetup }()
	cmdutils.SetupWorkspace = func(ctx context.Context, gitClient git.IClient, repoURL, workspace, ticketID, epicKey, timestamp string) (string, error) {
		setupCalled = true
		return repoURL, nil
	}

	cfg := SessionConfig{
		Logger: telemetry.NewLogger(true, "", false),
	}

	// Should return nil (skipped)
	err := ProcessJiraTicket(context.Background(), "BLOCKED-1", jClient, cfg, nil)
	assert.NoError(t, err)
	assert.False(t, setupCalled, "SetupWorkspace should not be called for blocked ticket")
}

func TestProcessJiraTicket_EpicDetection(t *testing.T) {
	// Mock Jira Server
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	mux.HandleFunc("/rest/api/3/issue/CHILD-1", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"key": "CHILD-1",
			"fields": map[string]interface{}{
				"summary": "Child Ticket",
				"parent": map[string]interface{}{
					"key": "EPIC-1",
				},
				"description": map[string]interface{}{
					"type": "doc", "version": 1,
					"content": []map[string]interface{}{
						{"type": "paragraph", "content": []map[string]interface{}{{"type": "text", "text": "Repo: https://github.com/foo/bar"}}},
					},
				},
			},
		})
	})

	mux.HandleFunc("/rest/api/3/issue/CHILD-1/transitions", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"transitions": []interface{}{
					map[string]interface{}{"id": "11", "name": "In Progress"},
				},
			})
		} else {
			w.WriteHeader(http.StatusNoContent)
		}
	})

	jClient := jira.NewClient(server.URL, "user", "token")

	// Track Epic Key passed to SetupWorkspace
	var capturedEpicKey string
	originalSetup := cmdutils.SetupWorkspace
	defer func() { cmdutils.SetupWorkspace = originalSetup }()
	cmdutils.SetupWorkspace = func(ctx context.Context, gitClient git.IClient, repoURL, workspace, ticketID, epicKey, timestamp string) (string, error) {
		capturedEpicKey = epicKey
		os.MkdirAll(workspace, 0755)
		return repoURL, nil
	}

	// Mock RunWorkflow to do nothing
	originalRun := RunWorkflow
	defer func() { RunWorkflow = originalRun }()
	RunWorkflow = func(ctx context.Context, cfg SessionConfig) error {
		return nil
	}

	cfg := SessionConfig{
		Cleanup: true,
		IsMock:  true,
	}

	err := ProcessJiraTicket(context.Background(), "CHILD-1", jClient, cfg, nil)
	assert.NoError(t, err)
	assert.Equal(t, "EPIC-1", capturedEpicKey)
}

func TestRunWorkflow_Detached_ExecutableError(t *testing.T) {
	// This covers the error path where executable finding fails or is messed up.
	// Hard to mock os.Executable but we can try to cover the failure by making it run Detached without name.
	cfg := SessionConfig{
		Detached:    true,
		SessionName: "", // Should error
	}
	err := RunWorkflow(context.Background(), cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "--name is required")
}
