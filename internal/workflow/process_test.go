package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"recac/internal/cmdutils"
	"recac/internal/git"
	"recac/internal/jira"
)

func TestProcess_DirectTask(t *testing.T) {
	// Backup original functions
	originalSetup := cmdutils.SetupWorkspace
	originalRun := RunWorkflow
	defer func() {
		cmdutils.SetupWorkspace = originalSetup
		RunWorkflow = originalRun
	}()

	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		// Mock SetupWorkspace
		cmdutils.SetupWorkspace = func(ctx context.Context, gitClient git.IClient, repoURL, workspace, ticketID, epicKey, timestamp string) (string, error) {
			return repoURL, nil
		}

		// Mock RunWorkflow
		RunWorkflow = func(ctx context.Context, cfg SessionConfig) error {
			return nil
		}

		cfg := SessionConfig{
			RepoURL: "https://github.com/test/repo",
			Summary: "Do something",
		}

		if err := ProcessDirectTask(ctx, cfg); err != nil {
			t.Errorf("expected success, got %v", err)
		}
	})

	t.Run("WorkspaceSetupFailure", func(t *testing.T) {
		cmdutils.SetupWorkspace = func(ctx context.Context, gitClient git.IClient, repoURL, workspace, ticketID, epicKey, timestamp string) (string, error) {
			return "", errors.New("setup failed")
		}

		cfg := SessionConfig{RepoURL: "https://github.com/test/repo"}
		if err := ProcessDirectTask(ctx, cfg); err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("RunWorkflowFailure", func(t *testing.T) {
		cmdutils.SetupWorkspace = func(ctx context.Context, gitClient git.IClient, repoURL, workspace, ticketID, epicKey, timestamp string) (string, error) {
			return repoURL, nil
		}
		RunWorkflow = func(ctx context.Context, cfg SessionConfig) error {
			return errors.New("workflow failed")
		}

		cfg := SessionConfig{RepoURL: "https://github.com/test/repo"}
		if err := ProcessDirectTask(ctx, cfg); err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("WriteAppSpec", func(t *testing.T) {
		tempDir, _ := os.MkdirTemp("", "test-workspace")
		defer os.RemoveAll(tempDir)

		cmdutils.SetupWorkspace = func(ctx context.Context, gitClient git.IClient, repoURL, workspace, ticketID, epicKey, timestamp string) (string, error) {
			// Simulate workspace setup by using the tempDir passed in cfg
			return repoURL, nil
		}
		RunWorkflow = func(ctx context.Context, cfg SessionConfig) error {
			return nil
		}

		cfg := SessionConfig{
			RepoURL: "https://github.com/test/repo",
			Summary: "Summary",
			Description: "Desc",
			ProjectPath: tempDir,
		}

		if err := ProcessDirectTask(ctx, cfg); err != nil {
			t.Errorf("expected success, got %v", err)
		}

		// Verify app_spec.txt
		content, err := os.ReadFile(filepath.Join(tempDir, "app_spec.txt"))
		if err != nil {
			t.Fatal("app_spec.txt not found")
		}
		if !strings.Contains(string(content), "Summary") {
			t.Error("app_spec.txt missing summary")
		}
	})
}

func TestProcess_JiraTicket(t *testing.T) {
	originalSetup := cmdutils.SetupWorkspace
	originalRun := RunWorkflow
	defer func() {
		cmdutils.SetupWorkspace = originalSetup
		RunWorkflow = originalRun
	}()

	ctx := context.Background()

	t.Run("TicketFetchError", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()
		client := jira.NewClient(server.URL, "u", "p")

		if err := ProcessJiraTicket(ctx, "PROJ-1", client, SessionConfig{}, nil); err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("BlockedTicket", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Respond to GetTicket
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"fields": map[string]interface{}{
					"issuelinks": []interface{}{
						map[string]interface{}{
							"type": map[string]interface{}{"inward": "is blocked by"},
							"inwardIssue": map[string]interface{}{
								"key": "BLK-1",
								"fields": map[string]interface{}{
									"status": map[string]interface{}{"name": "To Do"},
								},
							},
						},
					},
					"summary": "Blocked Ticket",
					"description": map[string]interface{}{
						"type": "doc",
						"content": []interface{}{},
					},
				},
			})
		}))
		defer server.Close()
		client := jira.NewClient(server.URL, "u", "p")

		// Should return nil (skipped)
		if err := ProcessJiraTicket(ctx, "PROJ-1", client, SessionConfig{}, nil); err != nil {
			t.Errorf("expected success (skip), got %v", err)
		}
	})

	t.Run("InvalidTicketFormat", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{}`)) // Missing "fields"
		}))
		defer server.Close()
		client := jira.NewClient(server.URL, "u", "p")

		if err := ProcessJiraTicket(ctx, "PROJ-1", client, SessionConfig{}, nil); err == nil {
			t.Error("expected error for invalid ticket format, got nil")
		}
	})

	t.Run("MissingRepoURL", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"fields": map[string]interface{}{
					"summary": "No Repo Ticket",
					"description": map[string]interface{}{
						"type": "doc",
						"content": []interface{}{},
					},
				},
			})
		}))
		defer server.Close()
		client := jira.NewClient(server.URL, "u", "p")

		if err := ProcessJiraTicket(ctx, "PROJ-1", client, SessionConfig{}, nil); err == nil {
			t.Error("expected error for missing repo url, got nil")
		}
	})

	t.Run("SetupWorkspaceFailure", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"fields": map[string]interface{}{
					"summary": "Valid Ticket",
					"description": map[string]interface{}{
						"type": "doc",
						"content": []interface{}{},
					},
				},
			})
		}))
		defer server.Close()
		client := jira.NewClient(server.URL, "u", "p")

		cmdutils.SetupWorkspace = func(ctx context.Context, gitClient git.IClient, repoURL, workspace, ticketID, epicKey, timestamp string) (string, error) {
			return "", errors.New("setup failed")
		}

		cfg := SessionConfig{RepoURL: "http://repo.git"}
		if err := ProcessJiraTicket(ctx, "PROJ-1", client, cfg, nil); err == nil {
			t.Error("expected error for setup failure, got nil")
		}
	})

	t.Run("Success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.Contains(r.URL.Path, "/transitions") {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			// GetTicket response
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"fields": map[string]interface{}{
					"summary": "Valid Ticket",
					"description": map[string]interface{}{
						"type": "doc",
						"content": []interface{}{},
					},
					"parent": map[string]interface{}{
						"key": "EPIC-1",
					},
				},
			})
		}))
		defer server.Close()
		client := jira.NewClient(server.URL, "u", "p")

		cmdutils.SetupWorkspace = func(ctx context.Context, gitClient git.IClient, repoURL, workspace, ticketID, epicKey, timestamp string) (string, error) {
			return repoURL, nil
		}
		RunWorkflow = func(ctx context.Context, cfg SessionConfig) error {
			if cfg.JiraEpicKey != "EPIC-1" {
				return errors.New("expected epic key EPIC-1")
			}
			return nil
		}

		cfg := SessionConfig{RepoURL: "http://repo.git", Cleanup: true} // Enable cleanup to cover defer
		if err := ProcessJiraTicket(ctx, "PROJ-1", client, cfg, nil); err != nil {
			t.Errorf("expected success, got %v", err)
		}
	})

	t.Run("RunWorkflowFailure", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.Contains(r.URL.Path, "/transitions") {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"fields": map[string]interface{}{
					"summary": "Valid Ticket",
					"description": map[string]interface{}{
						"type": "doc",
						"content": []interface{}{},
					},
				},
			})
		}))
		defer server.Close()
		client := jira.NewClient(server.URL, "u", "p")

		cmdutils.SetupWorkspace = func(ctx context.Context, gitClient git.IClient, repoURL, workspace, ticketID, epicKey, timestamp string) (string, error) {
			return repoURL, nil
		}
		RunWorkflow = func(ctx context.Context, cfg SessionConfig) error {
			return errors.New("workflow failed")
		}

		cfg := SessionConfig{RepoURL: "http://repo.git"}
		if err := ProcessJiraTicket(ctx, "PROJ-1", client, cfg, nil); err == nil {
			t.Error("expected error for workflow failure, got nil")
		}
	})
}
