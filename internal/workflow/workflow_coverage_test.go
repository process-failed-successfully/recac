package workflow

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"recac/internal/jira"
	"recac/internal/git"
	"recac/internal/cmdutils"
	"recac/internal/runner"
	"recac/internal/agent"

	"github.com/stretchr/testify/assert"
)

func TestProcessJiraTicket_Blocked(t *testing.T) {
	// Mock RunWorkflow
	originalRunWorkflow := RunWorkflow
	defer func() { RunWorkflow = originalRunWorkflow }()
	RunWorkflow = func(ctx context.Context, cfg SessionConfig) error {
		t.Error("RunWorkflow should not be called")
		return nil
	}

	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	// Mock Ticket
	mux.HandleFunc("/rest/api/3/issue/TEST-1", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"key": "TEST-1",
			"fields": map[string]interface{}{
				"summary": "Blocked Ticket",
				"description": map[string]interface{}{
					"type": "doc",
					"content": []interface{}{},
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
									"name": "Open",
								},
							},
						},
					},
				},
			},
		})
	})

	// Mock Blocker
	mux.HandleFunc("/rest/api/3/issue/BLOCKER-1", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"key": "BLOCKER-1",
			"fields": map[string]interface{}{
				"status": map[string]interface{}{
					"name": "Open",
				},
			},
		})
	})

	jClient := jira.NewClient(server.URL, "user", "token")
	cfg := SessionConfig{
		SessionName: "blocked-run",
		Logger:      nil,
	}

	err := ProcessJiraTicket(context.Background(), "TEST-1", jClient, cfg, nil)
	assert.NoError(t, err) // Returns nil (skipped)
}

func TestProcessJiraTicket_InvalidFormat(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	// Mock Ticket with missing fields
	mux.HandleFunc("/rest/api/3/issue/TEST-1", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"key": "TEST-1",
			// Missing fields
		})
	})

	jClient := jira.NewClient(server.URL, "user", "token")
	cfg := SessionConfig{
		SessionName: "invalid-run",
		Logger:      nil,
	}

	err := ProcessJiraTicket(context.Background(), "TEST-1", jClient, cfg, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid ticket format")
}

func TestRunWorkflow_DirtyWorkspace(t *testing.T) {
	tmpDir := t.TempDir()

	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = tmpDir
	assert.NoError(t, cmd.Run())

	// Configure git user
	cmd = exec.Command("git", "config", "user.email", "you@example.com")
	cmd.Dir = tmpDir
	assert.NoError(t, cmd.Run())
	cmd = exec.Command("git", "config", "user.name", "Your Name")
	cmd.Dir = tmpDir
	assert.NoError(t, cmd.Run())

	// Create a file
	err := os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte("content"), 0644)
	assert.NoError(t, err)

	// Add file (staged)
	cmd = exec.Command("git", "add", "test.txt")
	cmd.Dir = tmpDir
	assert.NoError(t, cmd.Run())

	// Commit initial change
	cmd = exec.Command("git", "commit", "-m", "initial")
	cmd.Dir = tmpDir
	assert.NoError(t, cmd.Run())

	// Modify file (dirty)
	err = os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte("modified"), 0644)
	assert.NoError(t, err)

	cfg := SessionConfig{
		ProjectPath: tmpDir,
		AllowDirty:  false,
		SessionName: "dirty-run",
		// We mock RunWorkflow to prevent it from continuing if check passes (which it shouldn't)
		// But RunWorkflow is called by RunWorkflow... wait, this test calls RunWorkflow directly.
	}

	// We need to mock NewSessionFunc to avoid panic if it gets there (should not)
	originalNewSessionFunc := NewSessionFunc
	defer func() { NewSessionFunc = originalNewSessionFunc }()
	NewSessionFunc = func(d runner.DockerClient, a agent.Agent, workspace, image, project, provider, model string, maxAgents int) *runner.Session {
		return nil
	}

	err = RunWorkflow(context.Background(), cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "uncommitted changes detected")
}

func TestProcessJiraTicket_ParentEpic(t *testing.T) {
	// Mock RunWorkflow
	originalRunWorkflow := RunWorkflow
	defer func() { RunWorkflow = originalRunWorkflow }()
	RunWorkflow = func(ctx context.Context, cfg SessionConfig) error {
		assert.Equal(t, "EPIC-123", cfg.JiraEpicKey)
		return nil
	}

	// Mock SetupWorkspace
	originalSetup := cmdutils.SetupWorkspace
	defer func() { cmdutils.SetupWorkspace = originalSetup }()
	cmdutils.SetupWorkspace = func(ctx context.Context, gitClient git.IClient, repoURL, workspace, ticketID, epicKey, timestamp string) (string, error) {
		os.MkdirAll(workspace, 0755)
		return repoURL, nil
	}

	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	// Mock Ticket with Parent
	mux.HandleFunc("/rest/api/3/issue/TEST-1", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"key": "TEST-1",
			"fields": map[string]interface{}{
				"summary": "Child Ticket",
				"description": map[string]interface{}{
					"type": "doc", "content": []interface{}{},
				},
				"parent": map[string]interface{}{
					"key": "EPIC-123",
				},
				"issuelinks": []interface{}{},
			},
		})
	})

	// Transitions
	mux.HandleFunc("/rest/api/3/issue/TEST-1/transitions", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			json.NewEncoder(w).Encode(map[string]interface{}{"transitions": []interface{}{}})
		} else {
			w.WriteHeader(http.StatusNoContent)
		}
	})


	jClient := jira.NewClient(server.URL, "user", "token")
	tmpDir := t.TempDir()

	cfg := SessionConfig{
		ProjectPath: tmpDir,
		SessionName: "child-run",
		RepoURL:     "https://github.com/example/repo",
		IsMock:      true,
	}

	err := ProcessJiraTicket(context.Background(), "TEST-1", jClient, cfg, nil)
	assert.NoError(t, err)
}
