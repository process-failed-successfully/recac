package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"recac/internal/cmdutils"
	"recac/internal/jira"
)

func TestJiraListCmd(t *testing.T) {
	// Mock Jira Server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/rest/api/3/search/jql") {
			// Check JQL
			jql := r.URL.Query().Get("jql")
			if !strings.Contains(jql, "assignee = currentUser()") {
				t.Errorf("Expected JQL to contain 'assignee = currentUser()', got '%s'", jql)
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			// Return mock issues
			response := map[string]interface{}{
				"issues": []map[string]interface{}{
					{
						"key": "PROJ-1",
						"fields": map[string]interface{}{
							"summary": "Test Ticket 1",
							"status": map[string]interface{}{
								"name": "Open",
							},
							"assignee": map[string]interface{}{
								"displayName": "Test User",
							},
						},
					},
				},
			}
			json.NewEncoder(w).Encode(response)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	// Override factory
	originalFactory := cmdutils.GetJiraClient
	defer func() { cmdutils.GetJiraClient = originalFactory }()

	cmdutils.GetJiraClient = func(ctx context.Context) (*jira.Client, error) {
		return jira.NewClient(server.URL, "user", "token"), nil
	}

	// Capture Output
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

    oldStderr := os.Stderr
    rErr, wErr, _ := os.Pipe()
    os.Stderr = wErr

	// Execute via Run function directly to bypass root hooks
	// Reset flags
	jiraListCmd.Flags().Set("jql", "")
	jiraListCmd.Flags().Set("status", "")
	jiraListCmd.Flags().Set("assigned-to-me", "true")

    if jiraListCmd.Run != nil {
	    jiraListCmd.Run(jiraListCmd, []string{})
    } else {
        t.Fatal("jiraListCmd.Run is nil")
    }

	w.Close()
    wErr.Close()
	os.Stdout = oldStdout
    os.Stderr = oldStderr

	// Read output
	out, _ := io.ReadAll(r)
	output := string(out)

    errOut, _ := io.ReadAll(rErr)
    stderr := string(errOut)

    if stderr != "" {
        t.Logf("Stderr: %s", stderr)
    }

	if !strings.Contains(output, "PROJ-1") {
		t.Errorf("Expected output to contain PROJ-1, got:\n%s\nStderr: %s", output, stderr)
	}
	if !strings.Contains(output, "Test Ticket 1") {
		t.Errorf("Expected output to contain 'Test Ticket 1', got:\n%s", output)
	}
}

func TestJiraCommentCmd(t *testing.T) {
	// Mock Jira Server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/rest/api/3/issue/PROJ-123/comment") {
			if r.Method != "POST" {
				t.Errorf("Expected POST, got %s", r.Method)
			}
			w.WriteHeader(http.StatusCreated)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	// Override factory
	originalFactory := cmdutils.GetJiraClient
	defer func() { cmdutils.GetJiraClient = originalFactory }()

	cmdutils.GetJiraClient = func(ctx context.Context) (*jira.Client, error) {
		return jira.NewClient(server.URL, "user", "token"), nil
	}

	// Capture Output
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

    oldStderr := os.Stderr
    rErr, wErr, _ := os.Pipe()
    os.Stderr = wErr

	// Execute via Run directly
    jiraCommentCmd.Flags().Set("id", "PROJ-123")
    jiraCommentCmd.Flags().Set("message", "This is a comment")

    if jiraCommentCmd.Run != nil {
	    jiraCommentCmd.Run(jiraCommentCmd, []string{"PROJ-123"})
    } else {
        t.Fatal("jiraCommentCmd.Run is nil")
    }

	w.Close()
    wErr.Close()
	os.Stdout = oldStdout
    os.Stderr = oldStderr

	// Read output
	out, _ := io.ReadAll(r)
	output := string(out)

    errOut, _ := io.ReadAll(rErr)
    stderr := string(errOut)

    if stderr != "" {
        t.Logf("Stderr: %s", stderr)
    }

	if !strings.Contains(output, "Success: Comment added to PROJ-123") {
		t.Errorf("Expected success message, got:\n%s\nStderr: %s", output, stderr)
	}
}
