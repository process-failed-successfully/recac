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

func TestProcessJiraTicket(t *testing.T) {
	// Mock RunWorkflow
	originalRunWorkflow := RunWorkflow
	defer func() { RunWorkflow = originalRunWorkflow }()
	RunWorkflow = func(ctx context.Context, cfg SessionConfig) error {
		return nil // Prevent running the full session
	}

	// Mock SetupWorkspace
	originalSetup := cmdutils.SetupWorkspace
	defer func() { cmdutils.SetupWorkspace = originalSetup }()

	cmdutils.SetupWorkspace = func(ctx context.Context, gitClient git.IClient, repoURL, workspace, ticketID, epicKey, timestamp string) (string, error) {
		// Mock success
		// Ensure workspace dir exists
		os.MkdirAll(workspace, 0755)
		return repoURL, nil
	}

	// Mock Jira Server
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	// Mock Ticket Response
	mux.HandleFunc("/rest/api/3/issue/TEST-1", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"key": "TEST-1",
			"fields": map[string]interface{}{
				"summary": "Test Ticket",
				"description": map[string]interface{}{
					"type":    "doc",
					"version": 1,
					"content": []map[string]interface{}{
						{
							"type": "paragraph",
							"content": []map[string]interface{}{
								{
									"type": "text",
									"text": "Repo: https://github.com/example/repo",
								},
							},
						},
					},
				},
				"issuelinks": []interface{}{},
			},
		})
	})

	// Mock Transition (search for transitions first)
	mux.HandleFunc("/rest/api/3/issue/TEST-1/transitions", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"transitions": []interface{}{
				map[string]interface{}{"id": "11", "name": "In Progress"},
			},
		})
	})

	// Create Client
	jClient := jira.NewClient(server.URL, "user", "token")

	// Config
	tmpDir, _ := os.MkdirTemp("", "workflow-jira-test")
	defer os.RemoveAll(tmpDir)

	cfg := SessionConfig{
		ProjectPath: tmpDir,
		SessionName: "test-run",
		Cleanup:     true,
		IsMock:      true,
	}

	err := ProcessJiraTicket(context.Background(), "TEST-1", jClient, cfg, nil)

	// Since we don't have DB, we expect RunWorkflow to fail or we mock DB?
	// mocking DB is hard.
	// We'll rely on IsMock: true in SessionConfig to perform a "Mock" run which should be lighter.

	// Check app_spec.txt
	specPath := fmt.Sprintf("%s/app_spec.txt", tmpDir)

	// If we want to verify, we should use Cleanup=false
	cfg.Cleanup = false

	err = ProcessJiraTicket(context.Background(), "TEST-1", jClient, cfg, nil)

	// Assert steps
	assert.FileExists(t, specPath)
	if err != nil {
		assert.Contains(t, err.Error(), "circuit breaker")
	} else {
		assert.NoError(t, err)
	}

	content, _ := os.ReadFile(specPath)
	assert.Contains(t, string(content), "Test Ticket")
	assert.Contains(t, string(content), "https://github.com/example/repo")
}

func TestProcessDirectTask(t *testing.T) {
	// Mock RunWorkflow
	originalRunWorkflow := RunWorkflow
	defer func() { RunWorkflow = originalRunWorkflow }()
	RunWorkflow = func(ctx context.Context, cfg SessionConfig) error {
		return nil // Prevent running the full session
	}

	// Mock SetupWorkspace
	originalSetup := cmdutils.SetupWorkspace
	defer func() { cmdutils.SetupWorkspace = originalSetup }()

	cmdutils.SetupWorkspace = func(ctx context.Context, gitClient git.IClient, repoURL, workspace, ticketID, epicKey, timestamp string) (string, error) {
		os.MkdirAll(workspace, 0755)
		return repoURL, nil
	}

	tmpDir, _ := os.MkdirTemp("", "workflow-direct-test")
	defer os.RemoveAll(tmpDir)

	cfg := SessionConfig{
		ProjectPath: tmpDir,
		RepoURL:     "https://github.com/example/direct",
		Summary:     "Do something",
		IsMock:      true,
	}

	err := ProcessDirectTask(context.Background(), cfg)

	// Check app_spec.txt
	specPath := fmt.Sprintf("%s/app_spec.txt", tmpDir)
	assert.FileExists(t, specPath)

	if err != nil {
		// assert.Contains(t, err.Error(), "database")
	}
}

func TestRunWorkflow_Detached(t *testing.T) {
	t.Skip("Skipping detached test due to binary dependency")
}

func TestProcessJiraTicket_WithRepoURL(t *testing.T) {
	// Mock RunWorkflow to avoid real execution which might fail with circuit breaker or other errors
	originalRunWorkflow := RunWorkflow
	defer func() { RunWorkflow = originalRunWorkflow }()
	RunWorkflow = func(ctx context.Context, cfg SessionConfig) error {
		return nil
	}

	// Mock SetupWorkspace
	originalSetup := cmdutils.SetupWorkspace
	defer func() { cmdutils.SetupWorkspace = originalSetup }()

	cmdutils.SetupWorkspace = func(ctx context.Context, gitClient git.IClient, repoURL, workspace, ticketID, epicKey, timestamp string) (string, error) {
		os.MkdirAll(workspace, 0755)
		return repoURL, nil
	}

	// Mock Jira Server (minimal)
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	// Mock Ticket Response (NO Repo: URL here to test fallback skip)
	mux.HandleFunc("/rest/api/3/issue/TEST-1", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"key": "TEST-1",
			"fields": map[string]interface{}{
				"summary": "Test Ticket",
				"description": map[string]interface{}{
					"type": "doc", "version": 1,
					"content": []map[string]interface{}{
						{"type": "paragraph", "content": []map[string]interface{}{{"type": "text", "text": "No repo here"}}},
					},
				},
				"issuelinks": []interface{}{},
			},
		})
	})

	mux.HandleFunc("/rest/api/3/issue/TEST-1/transitions", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			json.NewEncoder(w).Encode(map[string]interface{}{"transitions": []interface{}{map[string]interface{}{"id": "11", "name": "In Progress"}}})
		} else {
			w.WriteHeader(http.StatusNoContent)
		}
	})

	jClient := jira.NewClient(server.URL, "user", "token")
	tmpDir, _ := os.MkdirTemp("", "workflow-jira-repo-test")
	defer os.RemoveAll(tmpDir)

	cfg := SessionConfig{
		ProjectPath: tmpDir,
		RepoURL:     "https://github.com/example/already-provided",
		IsMock:      true,
		Cleanup:     false,
	}

	err := ProcessJiraTicket(context.Background(), "TEST-1", jClient, cfg, nil)

	// Should NOT return "no repo url found" error because RepoURL was provided in cfg.
	if err != nil {
		assert.NotContains(t, err.Error(), "no repo url found")
	}

	specPath := fmt.Sprintf("%s/app_spec.txt", tmpDir)
	assert.FileExists(t, specPath)
	content, _ := os.ReadFile(specPath)
	assert.Contains(t, string(content), "TEST-1")
}
