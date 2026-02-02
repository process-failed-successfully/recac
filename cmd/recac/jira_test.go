package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"recac/internal/cmdutils"
	"recac/internal/jira"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestJiraListCmd(t *testing.T) {
	// Mock Jira Server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/rest/api/3/search/jql" {
			// Check JQL param
			jql := r.URL.Query().Get("jql")
			if jql == "" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			// Return dummy issues
			resp := map[string]interface{}{
				"issues": []map[string]interface{}{
					{
						"key": "PROJ-1",
						"fields": map[string]interface{}{
							"summary": "Test Issue 1",
							"status": map[string]interface{}{
								"name": "To Do",
							},
							"assignee": map[string]interface{}{
								"displayName": "User 1",
							},
						},
					},
					{
						"key": "PROJ-2",
						"fields": map[string]interface{}{
							"summary": "Test Issue 2",
							"status": map[string]interface{}{
								"name": "In Progress",
							},
							"assignee": nil,
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

	// Override GetJiraClient to return client pointing to mock server
	originalGetJiraClient := cmdutils.GetJiraClient
	cmdutils.GetJiraClient = func(ctx context.Context) (*jira.Client, error) {
		return jira.NewClient(ts.URL, "user", "token"), nil
	}
	defer func() { cmdutils.GetJiraClient = originalGetJiraClient }()

	// Test Cases
	tests := []struct {
		name string
		args []string
		wantContains []string
	}{
		{
			name: "List default",
			args: []string{"jira", "list"},
			wantContains: []string{"PROJ-1", "Test Issue 1", "To Do", "User 1", "PROJ-2", "Unassigned"},
		},
		{
			name: "List with status",
			args: []string{"jira", "list", "--status", "Done"},
			wantContains: []string{"Searching with JQL: assignee = currentUser() AND status = \"Done\" ORDER BY updated DESC"},
		},
		{
			name: "List with explicit project",
			args: []string{"jira", "list", "--project", "TEST"},
			wantContains: []string{"Searching with JQL: assignee = currentUser() AND status != Done AND project = \"TEST\" ORDER BY updated DESC"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Execute
			output, err := executeCommand(rootCmd, tt.args...)
			assert.NoError(t, err)

			for _, want := range tt.wantContains {
				assert.Contains(t, output, want)
			}
		})
	}
}

func TestJiraCommentCmd(t *testing.T) {
	// Mock Jira Server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/rest/api/3/issue/PROJ-1/comment" && r.Method == "POST" {
			w.WriteHeader(http.StatusCreated)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	// Override GetJiraClient
	originalGetJiraClient := cmdutils.GetJiraClient
	cmdutils.GetJiraClient = func(ctx context.Context) (*jira.Client, error) {
		return jira.NewClient(ts.URL, "user", "token"), nil
	}
	defer func() { cmdutils.GetJiraClient = originalGetJiraClient }()

	// Test
	output, err := executeCommand(rootCmd, "jira", "comment", "--id", "PROJ-1", "--message", "Test Comment")
	assert.NoError(t, err)
	assert.Contains(t, output, "Comment added to PROJ-1")
}
