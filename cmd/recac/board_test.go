package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"recac/internal/cmdutils"
	"recac/internal/jira"

	"github.com/stretchr/testify/assert"
)

func TestBoardCmd(t *testing.T) {
	// 1. Mock Jira Server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/rest/api/3/search/jql" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			// Return mock issues
			resp := map[string]interface{}{
				"issues": []map[string]interface{}{
					{
						"key": "PROJ-1",
						"fields": map[string]interface{}{
							"summary": "Task 1",
							"status": map[string]interface{}{
								"name": "To Do",
								"statusCategory": map[string]interface{}{
									"name": "To Do",
								},
							},
						},
					},
					{
						"key": "PROJ-2",
						"fields": map[string]interface{}{
							"summary": "Task 2",
							"status": map[string]interface{}{
								"name": "In Progress",
								"statusCategory": map[string]interface{}{
									"name": "In Progress",
								},
							},
						},
					},
				},
			}
			json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	// 2. Override GetJiraClient
	originalFactory := cmdutils.GetJiraClient
	cmdutils.GetJiraClient = func(ctx context.Context) (*jira.Client, error) {
		return jira.NewClient(ts.URL, "user", "token"), nil
	}
	defer func() { cmdutils.GetJiraClient = originalFactory }()

	// 3. Set Test Env to skip TUI
	os.Setenv("RECAC_TEST_SKIP_TUI", "1")
	defer os.Unsetenv("RECAC_TEST_SKIP_TUI")

	// 4. Run Command
	// We execute via rootCmd to ensure proper command parsing and structure
	output, err := executeCommand(rootCmd, "board")
	assert.NoError(t, err)

	// 5. Assert Output
	assert.Contains(t, output, "Board initialized with 1 To Do, 1 In Progress, 0 Done")
}
