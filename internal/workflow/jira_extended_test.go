package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"recac/internal/cmdutils"
	"recac/internal/git"
	"recac/internal/jira"

	"github.com/stretchr/testify/assert"
)

func TestProcessJiraTicket_Blocked(t *testing.T) {
	// Mock SetupWorkspace to avoid actual git operations
	originalSetup := cmdutils.SetupWorkspace
	defer func() { cmdutils.SetupWorkspace = originalSetup }()
	cmdutils.SetupWorkspace = func(ctx context.Context, gitClient git.IClient, repoURL, workspace, ticketID, epicKey, timestamp string) (string, error) {
		return repoURL, nil
	}

	// Mock Jira Server
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	// Mock Ticket Response with Blockers
	mux.HandleFunc("/rest/api/3/issue/BLOCKED-1", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"key": "BLOCKED-1",
			"fields": map[string]interface{}{
				"summary": "Blocked Ticket",
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
									"name": "In Progress", // Not Done -> Blocker active
								},
							},
						},
					},
				},
			},
		})
	})

	jClient := jira.NewClient(server.URL, "user", "token")

	tmpDir := t.TempDir()
	cfg := SessionConfig{
		ProjectPath: tmpDir,
		IsMock:      true,
		Logger:      nil, // Will use default
	}

	// Should return nil (no error) but skip processing
	err := ProcessJiraTicket(context.Background(), "BLOCKED-1", jClient, cfg, nil)
	assert.NoError(t, err)

	// Verify workspace is empty as ticket was skipped (ProcessJiraTicket returns early if blocked)
	entries, _ := os.ReadDir(tmpDir)
	assert.Empty(t, entries, "Workspace should be empty as ticket was skipped")
}

func TestProcessDirectTask_WorkspaceError(t *testing.T) {
	// Mock SetupWorkspace to fail
	originalSetup := cmdutils.SetupWorkspace
	defer func() { cmdutils.SetupWorkspace = originalSetup }()
	cmdutils.SetupWorkspace = func(ctx context.Context, gitClient git.IClient, repoURL, workspace, ticketID, epicKey, timestamp string) (string, error) {
		return "", fmt.Errorf("git clone failed")
	}

	cfg := SessionConfig{
		RepoURL: "https://github.com/example/fail",
		Summary: "Fail",
		IsMock:  true,
	}

	err := ProcessDirectTask(context.Background(), cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "git clone failed")
}
