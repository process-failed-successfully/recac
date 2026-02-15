package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"recac/internal/agent"
	"recac/internal/cmdutils"
	"recac/internal/git"
	"recac/internal/jira"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRunWorkflow_GetAgentClientError(t *testing.T) {
	// Mock GetAgentClient to fail
	originalGetAgentClient := cmdutils.GetAgentClient
	defer func() { cmdutils.GetAgentClient = originalGetAgentClient }()

	cmdutils.GetAgentClient = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return nil, errors.New("agent init failed")
	}

	cfg := SessionConfig{
		SessionName: "test-agent-fail",
		IsMock:      false,
		ProjectPath: t.TempDir(),
		AllowDirty:  true, // skip git checks
	}

	err := RunWorkflow(context.Background(), cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to initialize agent")
	assert.Contains(t, err.Error(), "agent init failed")
}

func TestProcessJiraTicket_SetupWorkspaceError(t *testing.T) {
	// Mock Jira Client
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	mux.HandleFunc("/rest/api/3/issue/TEST-1", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"key": "TEST-1",
			"fields": map[string]interface{}{
				"summary": "Test Ticket",
				"description": map[string]interface{}{
					"type": "doc", "version": 1,
					"content": []map[string]interface{}{
						{"type": "paragraph", "content": []map[string]interface{}{{"type": "text", "text": "Repo: https://github.com/example/repo"}}},
					},
				},
				"issuelinks": []interface{}{},
			},
		})
	})

	jClient := jira.NewClient(server.URL, "user", "token")

	// Mock SetupWorkspace to fail
	originalSetup := cmdutils.SetupWorkspace
	defer func() { cmdutils.SetupWorkspace = originalSetup }()

	cmdutils.SetupWorkspace = func(ctx context.Context, gitClient git.IClient, repoURL, workspace, ticketID, epicKey, timestamp string) (string, error) {
		return "", errors.New("git clone failed")
	}

	// Mock RunWorkflow (should not be called)
	originalRunWorkflow := RunWorkflow
	defer func() { RunWorkflow = originalRunWorkflow }()
	RunWorkflow = func(ctx context.Context, cfg SessionConfig) error {
		t.Error("RunWorkflow should not be called")
		return nil
	}

	cfg := SessionConfig{
		RepoURL: "https://github.com/example/repo",
		IsMock:  true,
		Cleanup: true,
	}

	err := ProcessJiraTicket(context.Background(), "TEST-1", jClient, cfg, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "git clone failed")
}

func TestProcessDirectTask_SetupWorkspaceError(t *testing.T) {
	// Mock SetupWorkspace to fail
	originalSetup := cmdutils.SetupWorkspace
	defer func() { cmdutils.SetupWorkspace = originalSetup }()

	cmdutils.SetupWorkspace = func(ctx context.Context, gitClient git.IClient, repoURL, workspace, ticketID, epicKey, timestamp string) (string, error) {
		return "", errors.New("workspace setup failed")
	}

	cfg := SessionConfig{
		RepoURL:     "https://github.com/example/repo",
		ProjectPath: t.TempDir(),
	}

	err := ProcessDirectTask(context.Background(), cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "workspace setup failed")
}

func TestProcessDirectTask_RunWorkflowError(t *testing.T) {
	// Mock SetupWorkspace to succeed
	originalSetup := cmdutils.SetupWorkspace
	defer func() { cmdutils.SetupWorkspace = originalSetup }()

	cmdutils.SetupWorkspace = func(ctx context.Context, gitClient git.IClient, repoURL, workspace, ticketID, epicKey, timestamp string) (string, error) {
		os.MkdirAll(workspace, 0755)
		return repoURL, nil
	}

	// Mock RunWorkflow to fail
	originalRunWorkflow := RunWorkflow
	defer func() { RunWorkflow = originalRunWorkflow }()
	RunWorkflow = func(ctx context.Context, cfg SessionConfig) error {
		return errors.New("workflow failed")
	}

	cfg := SessionConfig{
		RepoURL:     "https://github.com/example/repo",
		ProjectPath: t.TempDir(),
		Summary:     "Test Summary",
	}

	err := ProcessDirectTask(context.Background(), cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "workflow failed")
}

func TestProcessDirectTask_WriteSpecError(t *testing.T) {
	// Mock SetupWorkspace to succeed
	originalSetup := cmdutils.SetupWorkspace
	defer func() { cmdutils.SetupWorkspace = originalSetup }()

	cmdutils.SetupWorkspace = func(ctx context.Context, gitClient git.IClient, repoURL, workspace, ticketID, epicKey, timestamp string) (string, error) {
		os.MkdirAll(workspace, 0755)
		// Make workspace read-only to fail WriteFile
		os.Chmod(workspace, 0555)
		return repoURL, nil
	}

	tmpDir := t.TempDir()
	// We need a subdir because SetupWorkspace might try to create it or similar
	workspace := filepath.Join(tmpDir, "read-only")

	cfg := SessionConfig{
		RepoURL:     "https://github.com/example/repo",
		ProjectPath: workspace,
		Summary:     "Test Summary",
	}

	// We need to ensure we can cleanup permissions to delete the temp dir?
	// t.TempDir automatically cleans up but might fail if read-only.
	// We should revert permissions in defer.
	defer func() {
		os.Chmod(workspace, 0755)
	}()

	err := ProcessDirectTask(context.Background(), cfg)
	assert.Error(t, err)
	// Error might be "permission denied" or similar
	assert.Contains(t, err.Error(), "permission denied")
}
