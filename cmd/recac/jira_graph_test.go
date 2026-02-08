package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"recac/internal/cmdutils"
	"recac/internal/jira"

	"github.com/stretchr/testify/assert"
)

func TestJiraGraphCmd(t *testing.T) {
	// Mock Jira API
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify auth is present (though we don't check value)
		_, _, ok := r.BasicAuth()
		if !ok {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		if r.URL.Path == "/rest/api/3/search/jql" {
			// Return mock issues
			// Issue 1 blocks Issue 2
			// Issue 2 is blocked by Issue 1

			// We need to return structure matching client.SearchIssues expectations
			// "issues": [...]
			// Each issue has "key", "fields": { "summary", "status", "issuelinks", "parent" }

			issues := []map[string]interface{}{
				{
					"key": "PROJ-1",
					"fields": map[string]interface{}{
						"summary": "Task 1",
						"status": map[string]interface{}{
							"name": "To Do",
						},
						"issuelinks": []interface{}{},
					},
				},
				{
					"key": "PROJ-2",
					"fields": map[string]interface{}{
						"summary": "Task 2 (Blocked by 1)",
						"status": map[string]interface{}{
							"name": "In Progress",
						},
						"issuelinks": []interface{}{
							map[string]interface{}{
								"type": map[string]interface{}{
									"inward": "is blocked by",
								},
								"inwardIssue": map[string]interface{}{
									"key": "PROJ-1",
									"fields": map[string]interface{}{
										"status": map[string]interface{}{
											"name": "To Do",
										},
									},
								},
							},
						},
					},
				},
				{
					"key": "PROJ-3",
					"fields": map[string]interface{}{
						"summary": "Task 3 (Done)",
						"status": map[string]interface{}{
							"name": "Done",
						},
						"issuelinks": []interface{}{},
					},
				},
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"issues": issues,
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	// Override factory
	origFactory := cmdutils.GetJiraClient
	defer func() { cmdutils.GetJiraClient = origFactory }()

	cmdutils.GetJiraClient = func(ctx context.Context) (*jira.Client, error) {
		return jira.NewClient(server.URL, "user", "token"), nil
	}

	t.Run("Mermaid Output", func(t *testing.T) {
		buf := new(bytes.Buffer)
		rootCmd.SetOut(buf)
		rootCmd.SetArgs([]string{"jira", "graph", "--project", "PROJ", "--format", "mermaid"})

		err := rootCmd.Execute()
		assert.NoError(t, err)

		output := buf.String()
		assert.Contains(t, output, "graph TD")
		// PROJ-1 is blocker for PROJ-2. So PROJ-1 --> PROJ-2
		assert.Contains(t, output, "PROJ_1 --> PROJ_2")

		// Verify styles
		assert.Contains(t, output, "PROJ_1[\"PROJ-1<br/>Task 1\"]:::pending")
		assert.Contains(t, output, "PROJ_2[\"PROJ-2<br/>Task 2 (Blocked by 1)\"]:::inprogress")
		assert.Contains(t, output, "PROJ_3[\"PROJ-3<br/>Task 3 (Done)\"]:::done")
	})

	t.Run("DOT Output", func(t *testing.T) {
		buf := new(bytes.Buffer)
		rootCmd.SetOut(buf)
		rootCmd.SetArgs([]string{"jira", "graph", "--project", "PROJ", "--format", "dot"})

		err := rootCmd.Execute()
		assert.NoError(t, err)

		output := buf.String()
		assert.Contains(t, output, "digraph G {")
		assert.Contains(t, output, "PROJ_1 -> PROJ_2;")
		assert.Contains(t, output, "fillcolor=\"lightgreen\"") // For Done items
	})

	// Test --include-done flag logic
	// PROJ-3 is Done. If we have a dependency on PROJ-3, it should be included regardless of flag?
	// The flag "include-done" usually means "include dependencies that are Done".
	// My implementation:
	// if !includeDone { if status is Done { continue } }
	// So if includeDone is false (default), dependencies on Done items are EXCLUDED.

	// Let's add a test case where PROJ-4 depends on PROJ-3 (Done).
	// If include-done=false, link PROJ-3 -> PROJ-4 should NOT exist.
	// If include-done=true, link PROJ-3 -> PROJ-4 SHOULD exist.
}
