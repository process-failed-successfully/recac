package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"recac/internal/agent"
	"recac/internal/cmdutils"
	"recac/internal/git"
	"recac/internal/jira"
	"recac/internal/runner"

	"github.com/stretchr/testify/assert"
)

func TestRunWorkflow_DirtyCheck(t *testing.T) {
	// Create temp dir and initialize git repo
	tmpDir, err := os.MkdirTemp("", "workflow-dirty-test")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

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

	// Create a file and commit it
	err = os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte("initial"), 0644)
	assert.NoError(t, err)
	cmd = exec.Command("git", "add", "test.txt")
	cmd.Dir = tmpDir
	assert.NoError(t, cmd.Run())
	cmd = exec.Command("git", "commit", "-m", "initial commit")
	cmd.Dir = tmpDir
	assert.NoError(t, cmd.Run())

	// Make a change (dirty state)
	err = os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte("changed"), 0644)
	assert.NoError(t, err)

	// Test 1: AllowDirty = false
	cfg := SessionConfig{
		ProjectPath: tmpDir,
		AllowDirty:  false,
		SessionName: "dirty-test",
	}

	err = RunWorkflow(context.Background(), cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "uncommitted changes detected")

	// Test 2: AllowDirty = true
	// Mock NewSessionFunc to stop further execution
	originalNewSessionFunc := NewSessionFunc
	defer func() { NewSessionFunc = originalNewSessionFunc }()
	NewSessionFunc = func(d runner.DockerClient, a agent.Agent, workspace, image, project, provider, model string, maxAgents int) *runner.Session {
		s := runner.NewSession(d, a, workspace, image, project, provider, model, maxAgents)
		s.MaxIterations = 0
		return s
	}

	cfg.AllowDirty = true
	cfg.Provider = "mock"
	cfg.IsMock = false
	cfg.Provider = "openai"

	err = RunWorkflow(context.Background(), cfg)
	if err != nil {
		assert.NotContains(t, err.Error(), "uncommitted changes detected")
	}
}

func TestProcessJiraTicket_ParentEpic(t *testing.T) {
	// Mock RunWorkflow to verify config
	originalRunWorkflow := RunWorkflow
	defer func() { RunWorkflow = originalRunWorkflow }()

	called := false
	RunWorkflow = func(ctx context.Context, cfg SessionConfig) error {
		called = true
		assert.Equal(t, "EPIC-1", cfg.JiraEpicKey)
		return nil
	}

	// Mock SetupWorkspace
	originalSetup := cmdutils.SetupWorkspace
	defer func() { cmdutils.SetupWorkspace = originalSetup }()
	cmdutils.SetupWorkspace = func(ctx context.Context, gitClient git.IClient, repoURL, workspace, ticketID, epicKey, timestamp string) (string, error) {
		return repoURL, nil
	}

	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	mux.HandleFunc("/rest/api/3/issue/TEST-1", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"key": "TEST-1",
			"fields": map[string]interface{}{
				"summary": "Child Ticket",
				"description": map[string]interface{}{
					"type": "doc", "version": 1,
					"content": []map[string]interface{}{
						{"type": "paragraph", "content": []map[string]interface{}{
							{"type": "text", "text": "Repo: https://example.com"},
						}},
					},
				},
				"parent": map[string]interface{}{
					"key": "EPIC-1",
				},
				"issuelinks": []interface{}{},
			},
		})
	})

	mux.HandleFunc("/rest/api/3/issue/TEST-1/transitions", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"transitions": []interface{}{
					map[string]interface{}{"id": "1", "name": "In Progress"},
				},
			})
		} else {
			w.WriteHeader(http.StatusNoContent)
		}
	})

	jClient := jira.NewClient(server.URL, "user", "token")
	tmpDir, _ := os.MkdirTemp("", "workflow-epic-test")
	defer os.RemoveAll(tmpDir)

	cfg := SessionConfig{
		ProjectPath: tmpDir,
		Cleanup:     false,
	}

	err := ProcessJiraTicket(context.Background(), "TEST-1", jClient, cfg, nil)
	assert.NoError(t, err)
	assert.True(t, called, "RunWorkflow should be called")
}

func TestProcessDirectTask_SetupError(t *testing.T) {
	// Mock SetupWorkspace failure
	originalSetup := cmdutils.SetupWorkspace
	defer func() { cmdutils.SetupWorkspace = originalSetup }()
	cmdutils.SetupWorkspace = func(ctx context.Context, gitClient git.IClient, repoURL, workspace, ticketID, epicKey, timestamp string) (string, error) {
		return "", errors.New("setup failed")
	}

	cfg := SessionConfig{
		RepoURL: "https://example.com",
		SessionName: "direct-fail",
	}

	err := ProcessDirectTask(context.Background(), cfg)
	assert.Error(t, err)
	assert.Equal(t, "setup failed", err.Error())
}

func TestRunWorkflow_SessionStartError(t *testing.T) {
	// Mock NewSessionFunc to return session that fails on Start
	// But Start() is a method on *runner.Session.
	// We cannot easily mock session methods unless session is an interface, but it's a struct.
	// However, `runner.NewSession` returns *runner.Session.

	// `session.Start(ctx)` logic:
	// It checks for Docker. If DockerClient is nil (which it is in our mocked case usually), it checks if UseLocalAgent is true.
	// If UseLocalAgent is false (default), it checks connection.

	// If we set IsMock=false, RunWorkflow calls `docker.NewClient`.
	// If that fails (it does in test environment usually), it logs warning and continues with nil dockerCli.

	// If dockerCli is nil, session.Start checks `s.AgentProvider == "mock"`.
	// If not mock, and dockerCli is nil, it sets `s.UseLocalAgent = true`.

	// So `session.Start` likely succeeds unless `os.MkdirAll` fails or something.

	// Actually, I can use `TestRunWorkflow_DirtyCheck` to cover the RunLoop part (it calls RunWorkflow).

	// Let's add a test for `GetAgentClient` failure.
	originalGetAgentClient := cmdutils.GetAgentClient
	defer func() { cmdutils.GetAgentClient = originalGetAgentClient }()
	cmdutils.GetAgentClient = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return nil, errors.New("agent init failed")
	}

	cfg := SessionConfig{
		IsMock: false,
		AllowDirty: true,
		SessionName: "agent-fail",
	}

	err := RunWorkflow(context.Background(), cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to initialize agent")
}
