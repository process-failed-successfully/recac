package workflow

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"recac/internal/cmdutils"
	"recac/internal/git"
	"recac/internal/jira"

	"github.com/stretchr/testify/assert"
)

func TestProcessJiraTicket_Blockers(t *testing.T) {
	// Mock SetupWorkspace
	originalSetup := cmdutils.SetupWorkspace
	defer func() { cmdutils.SetupWorkspace = originalSetup }()
	cmdutils.SetupWorkspace = func(ctx context.Context, gitClient git.IClient, repoURL, workspace, ticketID, epicKey, timestamp string) (string, error) {
		return repoURL, nil
	}

	// Mock Jira Server
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	// Mock Ticket with Blockers
	mux.HandleFunc("/rest/api/3/issue/BLOCKED-1", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"key": "BLOCKED-1",
			"fields": map[string]interface{}{
				"summary": "Blocked Ticket",
				"description": map[string]interface{}{
					"type": "doc",
					"content": []interface{}{},
				},
				"issuelinks": []interface{}{
					map[string]interface{}{
						"type": map[string]interface{}{
							"name": "Blocks",
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
	cfg := SessionConfig{
		SessionName: "test-blocked",
		IsMock:      true,
		Logger:      nil, // Will be initialized
	}

	// Should return nil (skipped)
	err := ProcessJiraTicket(context.Background(), "BLOCKED-1", jClient, cfg, nil)
	assert.NoError(t, err)
}

func TestProcessJiraTicket_EpicParent(t *testing.T) {
	// Mock RunWorkflow to capture config
	originalRunWorkflow := RunWorkflow
	defer func() { RunWorkflow = originalRunWorkflow }()

	var capturedCfg SessionConfig
	RunWorkflow = func(ctx context.Context, cfg SessionConfig) error {
		capturedCfg = cfg
		return nil
	}

	// Mock SetupWorkspace
	originalSetup := cmdutils.SetupWorkspace
	defer func() { cmdutils.SetupWorkspace = originalSetup }()
	cmdutils.SetupWorkspace = func(ctx context.Context, gitClient git.IClient, repoURL, workspace, ticketID, epicKey, timestamp string) (string, error) {
		os.MkdirAll(workspace, 0755)
		return repoURL, nil
	}

	// Mock Jira Server
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	// Mock Ticket with Parent
	mux.HandleFunc("/rest/api/3/issue/CHILD-1", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"key": "CHILD-1",
			"fields": map[string]interface{}{
				"summary": "Child Ticket",
				"description": map[string]interface{}{
					"type": "doc",
					"content": []interface{}{
						map[string]interface{}{
							"type": "paragraph",
							"content": []interface{}{
								map[string]interface{}{
									"type": "text",
									"text": "Repo: https://example.com",
								},
							},
						},
					},
				},
				"parent": map[string]interface{}{
					"key": "EPIC-1",
				},
				"issuelinks": []interface{}{},
			},
		})
	})

	mux.HandleFunc("/rest/api/3/issue/CHILD-1/transitions", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	jClient := jira.NewClient(server.URL, "user", "token")

	tmpDir, _ := os.MkdirTemp("", "workflow-epic-test")
	defer os.RemoveAll(tmpDir)

	cfg := SessionConfig{
		ProjectPath: tmpDir,
		SessionName: "test-epic",
		IsMock:      true,
		Cleanup:     false,
	}

	err := ProcessJiraTicket(context.Background(), "CHILD-1", jClient, cfg, nil)
	assert.NoError(t, err)

	// Verify Epic Key was captured
	assert.Equal(t, "EPIC-1", capturedCfg.JiraEpicKey)
}
