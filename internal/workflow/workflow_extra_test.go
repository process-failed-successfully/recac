package workflow

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"recac/internal/jira"
	"recac/internal/cmdutils"
	"recac/internal/git"

	"github.com/stretchr/testify/assert"
)

func TestProcessJiraTicket_Blocked(t *testing.T) {
	// Mock Jira Server
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	// Mock Ticket Response (Blocked by BLOCKED-1 which is In Progress)
	mux.HandleFunc("/rest/api/3/issue/TEST-1", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"key": "TEST-1",
			"fields": map[string]interface{}{
				"summary": "Test Ticket",
				"description": map[string]interface{}{
					"type": "doc", "version": 1,
					"content": []interface{}{}, // Empty description
				},
				"issuelinks": []interface{}{
					map[string]interface{}{
						"type": map[string]interface{}{
							"inward": "is blocked by",
						},
						"inwardIssue": map[string]interface{}{
							"key": "BLOCKED-1",
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

	cfg := SessionConfig{
		Logger: nil, // Will use default
	}

	// Execution
	// RunWorkflow should NOT be called if ticket is blocked.
	// But to be sure, we can mock RunWorkflow to panic or fail if called.
	originalRunWorkflow := RunWorkflow
	defer func() { RunWorkflow = originalRunWorkflow }()
	RunWorkflow = func(ctx context.Context, cfg SessionConfig) error {
		t.Fatal("RunWorkflow should not be called for blocked ticket")
		return nil
	}

	err := ProcessJiraTicket(context.Background(), "TEST-1", jClient, cfg, nil)

	// Assertions
	assert.NoError(t, err)
}

func TestProcessJiraTicket_IgnoredBlocker(t *testing.T) {
	// Mock Jira Server
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	// Mock Ticket Response (Blocked by BLOCKED-1 which is In Progress)
	mux.HandleFunc("/rest/api/3/issue/TEST-1", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"key": "TEST-1",
			"fields": map[string]interface{}{
				"summary": "Test Ticket",
				"description": map[string]interface{}{
					"type": "doc", "version": 1,
					"content": []interface{}{},
				},
				"issuelinks": []interface{}{
					map[string]interface{}{
						"type": map[string]interface{}{
							"inward": "is blocked by",
						},
						"inwardIssue": map[string]interface{}{
							"key": "BLOCKED-1",
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

	// Mock Transition (needed because now it proceeds)
	mux.HandleFunc("/rest/api/3/issue/TEST-1/transitions", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			json.NewEncoder(w).Encode(map[string]interface{}{"transitions": []interface{}{map[string]interface{}{"id": "11", "name": "In Progress"}}})
		} else {
			w.WriteHeader(http.StatusNoContent)
		}
	})

	jClient := jira.NewClient(server.URL, "user", "token")

	// Mock SetupWorkspace
	originalSetup := cmdutils.SetupWorkspace
	defer func() { cmdutils.SetupWorkspace = originalSetup }()
	cmdutils.SetupWorkspace = func(ctx context.Context, gitClient git.IClient, repoURL, workspace, ticketID, epicKey, timestamp string) (string, error) {
		return repoURL, nil
	}

	// Mock RunWorkflow to do nothing (success)
	originalRunWorkflow := RunWorkflow
	defer func() { RunWorkflow = originalRunWorkflow }()
	called := false
	RunWorkflow = func(ctx context.Context, cfg SessionConfig) error {
		called = true
		return nil
	}

	cfg := SessionConfig{
		ProjectPath: os.TempDir(),
		RepoURL: "http://github.com/example/repo", // Provide repo URL to skip description parsing
		Cleanup: true,
	}

	ignored := map[string]bool{"BLOCKED-1": true}
	err := ProcessJiraTicket(context.Background(), "TEST-1", jClient, cfg, ignored)

	assert.NoError(t, err)
	assert.True(t, called, "RunWorkflow should be called when blocker is ignored")
}
