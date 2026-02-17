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
	"recac/internal/runner"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestProcessDirectTask_ErrorPaths(t *testing.T) {
	// Mock SetupWorkspace to fail
	originalSetup := cmdutils.SetupWorkspace
	defer func() { cmdutils.SetupWorkspace = originalSetup }()
	cmdutils.SetupWorkspace = func(ctx context.Context, gitClient git.IClient, repoURL, workspace, ticketID, epicKey, timestamp string) (string, error) {
		return "", errors.New("setup failed")
	}

	cfg := SessionConfig{
		ProjectPath: "/tmp/test-project",
		RepoURL:     "https://github.com/example/repo",
		IsMock:      true,
	}

	err := ProcessDirectTask(context.Background(), cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "setup failed")
}

func TestRunWorkflow_AgentClientFailure(t *testing.T) {
	// Mock GetAgentClient to fail
	originalGetAgentClient := cmdutils.GetAgentClient
	defer func() { cmdutils.GetAgentClient = originalGetAgentClient }()
	cmdutils.GetAgentClient = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return nil, errors.New("agent init failed")
	}

	tmpDir, _ := os.MkdirTemp("", "workflow-agent-fail")
	defer os.RemoveAll(tmpDir)

	// We need app_spec.txt or AllowDirty
	cfg := SessionConfig{
		ProjectPath: tmpDir,
		SessionName: "test-run",
		IsMock:      false, // Normal mode to trigger Agent init
		AllowDirty:  true,
	}

	err := RunWorkflow(context.Background(), cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "agent init failed")
}

func TestProcessJiraTicket_Cleanup(t *testing.T) {
	// Mock SetupWorkspace to succeed
	originalSetup := cmdutils.SetupWorkspace
	defer func() { cmdutils.SetupWorkspace = originalSetup }()
	cmdutils.SetupWorkspace = func(ctx context.Context, gitClient git.IClient, repoURL, workspace, ticketID, epicKey, timestamp string) (string, error) {
		os.MkdirAll(workspace, 0755)
		return repoURL, nil
	}

	// Mock RunWorkflow
	originalRunWorkflow := RunWorkflow
	defer func() { RunWorkflow = originalRunWorkflow }()
	RunWorkflow = func(ctx context.Context, cfg SessionConfig) error {
		return nil
	}

	// Mock Jira Client
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	mux.HandleFunc("/rest/api/3/issue/CLEAN-1", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"key": "CLEAN-1",
			"fields": map[string]interface{}{
				"summary": "Clean Ticket",
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

	mux.HandleFunc("/rest/api/3/issue/CLEAN-1/transitions", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	jClient := jira.NewClient(server.URL, "user", "token")

	// Use a specific temp dir that we can check existence of
	tmpBase, _ := os.MkdirTemp("", "workflow-clean-test-base")
	defer os.RemoveAll(tmpBase)

	workspace := filepath.Join(tmpBase, "workspace")

	cfg := SessionConfig{
		ProjectPath: workspace,
		Cleanup: true,
		IsMock: true,
	}

	err := ProcessJiraTicket(context.Background(), "CLEAN-1", jClient, cfg, nil)
	assert.NoError(t, err)

	// Check if workspace is gone
	_, err = os.Stat(workspace)
	if !os.IsNotExist(err) {
		t.Error("Workspace should have been cleaned up")
	}
}

func TestRunWorkflow_Detached_AllFlags(t *testing.T) {
	mockSM := new(MockSessionManager)
	mockSM.On("StartSession", "test-flags", "goal", mock.MatchedBy(func(cmd []string) bool {
		// Check for all flags
		found := 0
		for i, arg := range cmd {
			if arg == "--mock" { found++ }
			if arg == "--allow-dirty" { found++ }
			if arg == "--max-iterations" && cmd[i+1] == "100" { found++ }
			if arg == "--manager-frequency" && cmd[i+1] == "10" { found++ }
			if arg == "--task-max-iterations" && cmd[i+1] == "5" { found++ }
			if arg == "--path" && cmd[i+1] == "/tmp/path" { found++ }
		}
		return found >= 6
	}), "/tmp/path").Return(&runner.SessionState{PID: 1}, nil).Once()

	cfg := SessionConfig{
		Detached:          true,
		SessionName:       "test-flags",
		Goal:              "goal",
		SessionManager:    mockSM,
		IsMock:            true,
		AllowDirty:        true,
		MaxIterations:     100,
		ManagerFrequency:  10,
		TaskMaxIterations: 5,
		ProjectPath:       "/tmp/path",
	}

	err := RunWorkflow(context.Background(), cfg)
	assert.NoError(t, err)
	mockSM.AssertExpectations(t)
}

func TestProcessJiraTicket_RunWorkflowFail(t *testing.T) {
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

	// Mock Jira Client (minimal)
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	mux.HandleFunc("/rest/api/3/issue/FAIL-1", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"key": "FAIL-1",
			"fields": map[string]interface{}{
				"summary": "Fail Ticket",
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
	mux.HandleFunc("/rest/api/3/issue/FAIL-1/transitions", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	jClient := jira.NewClient(server.URL, "user", "token")

	tmpDir := os.TempDir()
	cfg := SessionConfig{
		ProjectPath: tmpDir,
		IsMock: true,
	}

	err := ProcessJiraTicket(context.Background(), "FAIL-1", jClient, cfg, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "workflow failed")
}

func TestProcessDirectTask_RunWorkflowFail(t *testing.T) {
	// Mock SetupWorkspace to succeed
	originalSetup := cmdutils.SetupWorkspace
	defer func() { cmdutils.SetupWorkspace = originalSetup }()
	cmdutils.SetupWorkspace = func(ctx context.Context, gitClient git.IClient, repoURL, workspace, ticketID, epicKey, timestamp string) (string, error) {
		return repoURL, nil
	}

	// Mock RunWorkflow to fail
	originalRunWorkflow := RunWorkflow
	defer func() { RunWorkflow = originalRunWorkflow }()
	RunWorkflow = func(ctx context.Context, cfg SessionConfig) error {
		return errors.New("workflow failed")
	}

	cfg := SessionConfig{
		RepoURL: "https://github.com/example/repo",
		IsMock: true,
	}

	err := ProcessDirectTask(context.Background(), cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "workflow failed")
}
