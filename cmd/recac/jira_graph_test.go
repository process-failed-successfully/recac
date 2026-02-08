package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"recac/internal/cmdutils"
	"recac/internal/jira"
	"strings"
	"testing"
)

func TestJiraGraphCmd(t *testing.T) {
	// Mock Jira Server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Mock Authentication
		if strings.Contains(r.URL.Path, "/myself") {
			w.WriteHeader(http.StatusOK)
			return
		}
		// Mock Search
		if strings.Contains(r.URL.Path, "/search") {
			response := map[string]interface{}{
				"issues": []map[string]interface{}{
					{
						"key": "PROJ-1",
						"fields": map[string]interface{}{
							"summary": "Blocker Task",
							"status": map[string]interface{}{
								"name": "Done",
							},
							"issuelinks": []interface{}{},
						},
					},
					{
						"key": "PROJ-2",
						"fields": map[string]interface{}{
							"summary": "Blocked Task",
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
												"name": "Done",
											},
										},
									},
								},
							},
						},
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
			return
		}
		http.NotFound(w, r)
	}))
	defer ts.Close()

	// Override GetJiraClient
	originalGetJiraClient := cmdutils.GetJiraClient
	defer func() { cmdutils.GetJiraClient = originalGetJiraClient }()

	cmdutils.GetJiraClient = func(ctx context.Context) (*jira.Client, error) {
		return jira.NewClient(ts.URL, "user", "token"), nil
	}

	t.Run("Mermaid Output", func(t *testing.T) {
		output, err := executeCommand(rootCmd, "jira", "graph", "--project", "PROJ")
		if err != nil {
			t.Fatalf("Command failed: %v", err)
		}

		// Check for Mermaid syntax
		if !strings.Contains(output, "graph TD") {
			t.Errorf("Expected 'graph TD', got:\n%s", output)
		}
		// Check for nodes (sanitized)
		if !strings.Contains(output, "PROJ_1") || !strings.Contains(output, "PROJ_2") {
			t.Errorf("Expected nodes PROJ_1 and PROJ_2, got:\n%s", output)
		}
		// Check for edge
		if !strings.Contains(output, "PROJ_1 --> PROJ_2") {
			t.Errorf("Expected edge PROJ_1 --> PROJ_2, got:\n%s", output)
		}
		// Check styles
		if !strings.Contains(output, ":::done") {
			t.Errorf("Expected ':::done' style, got:\n%s", output)
		}
	})

	t.Run("DOT Output", func(t *testing.T) {
		output, err := executeCommand(rootCmd, "jira", "graph", "--project", "PROJ", "--output", "dot")
		if err != nil {
			t.Fatalf("Command failed: %v", err)
		}

		if !strings.Contains(output, "digraph G {") {
			t.Errorf("Expected 'digraph G {', got:\n%s", output)
		}
		// DOT uses quotes for IDs, so original keys are preserved
		if !strings.Contains(output, "\"PROJ-1\" -> \"PROJ-2\";") {
			t.Errorf("Expected edge \"PROJ-1\" -> \"PROJ-2\", got:\n%s", output)
		}
	})
}
