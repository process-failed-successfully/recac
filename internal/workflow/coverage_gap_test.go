package workflow

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"recac/internal/agent"
	"recac/internal/cmdutils"
	"recac/internal/git"
	"recac/internal/jira"
	"recac/internal/runner"
	"recac/internal/telemetry"

	"github.com/stretchr/testify/assert"
)

func TestProcessJiraTicket_Blocked(t *testing.T) {
	// Mock SetupWorkspace to do nothing
	originalSetup := cmdutils.SetupWorkspace
	defer func() { cmdutils.SetupWorkspace = originalSetup }()
	cmdutils.SetupWorkspace = func(ctx context.Context, gitClient git.IClient, repoURL, workspace, ticketID, epicKey, timestamp string) (string, error) {
		return repoURL, nil
	}

	// Mock Jira Server
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	// Mock Ticket Response with Blocker
	mux.HandleFunc("/rest/api/3/issue/BLOCKED-1", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"key": "BLOCKED-1",
			"fields": map[string]interface{}{
				"summary": "Blocked Ticket",
				"issuelinks": []interface{}{
					map[string]interface{}{
						"type": map[string]interface{}{"inward": "is blocked by"},
						"inwardIssue": map[string]interface{}{
							"key":    "BLOCKER-1",
							"fields": map[string]interface{}{"status": map[string]interface{}{"name": "In Progress"}},
						},
					},
				},
			},
		})
	})

	jClient := jira.NewClient(server.URL, "user", "token")
	cfg := SessionConfig{
		IsMock: true,
		Logger: nil, // Should default to new logger
	}

	// Should return nil (no error) and skip
	err := ProcessJiraTicket(context.Background(), "BLOCKED-1", jClient, cfg, nil)
	assert.NoError(t, err)
}

func TestProcessJiraTicket_Cleanup(t *testing.T) {
	// Mock RunWorkflow to just return
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

	// Mock Jira Server
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	mux.HandleFunc("/rest/api/3/issue/CLEAN-1", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"key": "CLEAN-1",
			"fields": map[string]interface{}{
				"summary": "Cleanup Ticket",
				"description": map[string]interface{}{
					"type": "doc", "version": 1,
					"content": []map[string]interface{}{
						{"type": "paragraph", "content": []map[string]interface{}{{"type": "text", "text": "Repo: https://github.com/example/repo"}}},
					},
				},
			},
		})
	})

	mux.HandleFunc("/rest/api/3/issue/CLEAN-1/transitions", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"transitions": []interface{}{
				map[string]interface{}{"id": "11", "name": "In Progress"},
			},
		})
	})

	jClient := jira.NewClient(server.URL, "user", "token")

	// We can't easily predict the temp dir name created inside ProcessJiraTicket unless we mock os.MkdirTemp or provide ProjectPath.
	// But if we provide ProjectPath, the cleanup logic inside ProcessJiraTicket depends on `cfg.Cleanup`.
	// The code says:
	// if cfg.Cleanup { defer func() { os.RemoveAll(tempWorkspace) }() }

	// So if we provide a ProjectPath, it should be removed.

	tmpDir, _ := os.MkdirTemp("", "test-cleanup-verify")
	// We don't defer remove here because we want to check if it's removed by the function.

	cfg := SessionConfig{
		ProjectPath: tmpDir,
		Cleanup:     true,
		IsMock:      true,
	}

	err := ProcessJiraTicket(context.Background(), "CLEAN-1", jClient, cfg, nil)
	assert.NoError(t, err)

	// Check if tmpDir exists
	_, err = os.Stat(tmpDir)
	assert.True(t, os.IsNotExist(err), "Workspace should be removed")

	// Cleanup just in case
	if !os.IsNotExist(err) {
		os.RemoveAll(tmpDir)
	}
}

func TestProcessJiraTicket_Transition(t *testing.T) {
	originalRunWorkflow := RunWorkflow
	defer func() { RunWorkflow = originalRunWorkflow }()
	RunWorkflow = func(ctx context.Context, cfg SessionConfig) error { return nil }

	originalSetup := cmdutils.SetupWorkspace
	defer func() { cmdutils.SetupWorkspace = originalSetup }()
	cmdutils.SetupWorkspace = func(ctx context.Context, gitClient git.IClient, repoURL, workspace, ticketID, epicKey, timestamp string) (string, error) {
		os.MkdirAll(workspace, 0755)
		return repoURL, nil
	}

	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	mux.HandleFunc("/rest/api/3/issue/TRANS-1", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"key": "TRANS-1",
			"fields": map[string]interface{}{
				"summary": "Transition Ticket",
				"description": map[string]interface{}{
					"type": "doc", "version": 1,
					"content": []map[string]interface{}{
						{"type": "paragraph", "content": []map[string]interface{}{{"type": "text", "text": "Repo: https://github.com/example/repo"}}},
					},
				},
			},
		})
	})

	transitionCalled := false
	mux.HandleFunc("/rest/api/3/issue/TRANS-1/transitions", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"transitions": []interface{}{
					map[string]interface{}{"id": "11", "name": "In Progress"},
				},
			})
		} else if r.Method == "POST" {
			transitionCalled = true
			w.WriteHeader(http.StatusNoContent)
		}
	})

	jClient := jira.NewClient(server.URL, "user", "token")
	cfg := SessionConfig{
		ProjectPath: t.TempDir(),
		IsMock:      true,
	}

	err := ProcessJiraTicket(context.Background(), "TRANS-1", jClient, cfg, nil)
	assert.NoError(t, err)
	assert.True(t, transitionCalled, "SmartTransition should be called")
}

type MockNotifier struct{}

func (m *MockNotifier) Start(ctx context.Context) {}
func (m *MockNotifier) Notify(ctx context.Context, eventType string, message string, threadTS string) (string, error) {
	return "ts-123", nil
}
func (m *MockNotifier) AddReaction(ctx context.Context, timestamp, reaction string) error {
	return nil
}

func TestRunWorkflow_MockMode_Config(t *testing.T) {
	// Mock NewSessionFunc to capture config
	originalNewSession := NewSessionFunc
	defer func() { NewSessionFunc = originalNewSession }()

	var capturedSession *runner.Session
	NewSessionFunc = func(d runner.DockerClient, a agent.Agent, workspace, image, project, provider, model string, maxAgents int) *runner.Session {
		s := &runner.Session{
			Docker:           d,
			Agent:            a,
			Workspace:        workspace,
			Image:            image,
			Project:          project,
			AgentProvider:    provider,
			AgentModel:       model,
			MaxAgents:        maxAgents,
			SpecFile:         "app_spec.txt",
			MaxIterations:    20,
			ManagerFrequency: 5,
			Notifier:         &MockNotifier{},
			Logger:           telemetry.NewLogger(true, "", false),
		}
		capturedSession = s
		// Prevent RunLoop from actually running long
		s.MaxIterations = 0
		return s
	}

	// Mock cmdutils.GetAgentClient not needed for Mock Mode as it uses NewMockAgent directly

	cfg := SessionConfig{
		IsMock:           true,
		SessionName:      "mock-config-test",
		MaxIterations:    42,
		TaskMaxIterations: 7,
		ManagerFrequency: 3,
		Stream:           true,
		AutoMerge:        true,
		SkipQA:           true,
		ManagerFirst:     true,
		JiraEpicKey:      "EPIC-123",
	}

	err := RunWorkflow(context.Background(), cfg)

	if err != nil && err.Error() != "circuit breaker: no-op loop" && err.Error() != "maximum iterations reached" {
		// ignore
	}

	assert.NotNil(t, capturedSession)
	assert.Equal(t, 42, capturedSession.MaxIterations)
	assert.Equal(t, 7, capturedSession.TaskMaxIterations)
	assert.Equal(t, 3, capturedSession.ManagerFrequency)
	assert.True(t, capturedSession.StreamOutput)
	assert.True(t, capturedSession.AutoMerge)
	assert.True(t, capturedSession.SkipQA)
	assert.True(t, capturedSession.ManagerFirst)
	assert.Equal(t, "agent-epic/EPIC-123", capturedSession.BaseBranch)
}

func TestRunWorkflow_Detached_Executable(t *testing.T) {
    // This test attempts to cover the fallback logic when finding executable.
    // It's hard to mock os.Executable, but we can verify that providing a CommandPrefix
    // changes the command passed to SessionManager.

    mockSM := &ManualMockSessionManager{
        StartSessionFunc: func(name, goal string, command []string, cwd string) (*runner.SessionState, error) {
            // Verify command contains custom prefix
            found := false
            for _, arg := range command {
                if arg == "custom-start" {
                    found = true
                    break
                }
            }
            assert.True(t, found, "Command should contain custom prefix")
            return &runner.SessionState{PID: 1}, nil
        },
    }

    cfg := SessionConfig{
        Detached:      true,
        SessionName:   "detached-exec",
        CommandPrefix: []string{"custom-start"},
        SessionManager: mockSM,
    }

    err := RunWorkflow(context.Background(), cfg)
    assert.NoError(t, err)
}

func TestProcessDirectTask_SetupWorkspaceError(t *testing.T) {
	// Mock SetupWorkspace to fail
	originalSetup := cmdutils.SetupWorkspace
	defer func() { cmdutils.SetupWorkspace = originalSetup }()
	cmdutils.SetupWorkspace = func(ctx context.Context, gitClient git.IClient, repoURL, workspace, ticketID, epicKey, timestamp string) (string, error) {
		return "", assert.AnError
	}

	cfg := SessionConfig{
		RepoURL: "https://github.com/example/fail",
		IsMock:  true,
	}

	err := ProcessDirectTask(context.Background(), cfg)
	assert.Error(t, err)
	assert.Equal(t, assert.AnError, err)
}

func TestProcessDirectTask_WriteSpecError(t *testing.T) {
	// Mock SetupWorkspace to succeed
	originalSetup := cmdutils.SetupWorkspace
	defer func() { cmdutils.SetupWorkspace = originalSetup }()
	cmdutils.SetupWorkspace = func(ctx context.Context, gitClient git.IClient, repoURL, workspace, ticketID, epicKey, timestamp string) (string, error) {
		return repoURL, nil
	}

	// Use a read-only directory to force WriteFile error
	tmpDir := t.TempDir()
	os.Chmod(tmpDir, 0400) // Read-only

	cfg := SessionConfig{
		ProjectPath: tmpDir,
		Summary:     "Test",
		Description: "Desc",
		IsMock:      true,
	}

	err := ProcessDirectTask(context.Background(), cfg)
	// On some systems root can still write, so check if error happened
	assert.Error(t, err, "Expected write error")
	if err != nil {
		assert.Contains(t, err.Error(), "permission denied")
	}
}

func TestRunWorkflow_AgentInitFail(t *testing.T) {
	// Mock cmdutils.GetAgentClient to fail
	originalGetAgentClient := cmdutils.GetAgentClient
	defer func() { cmdutils.GetAgentClient = originalGetAgentClient }()
	cmdutils.GetAgentClient = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return nil, assert.AnError
	}

	// Mock Docker to succeed (so we reach agent init)
	// We need a real docker client mock logic?
	// NewSessionFunc is NOT mocked here because we want to test RunWorkflow logic BEFORE NewSessionFunc

	// Create a temp dir with .git to pass pre-flight
	tmpDir := t.TempDir()
	os.Mkdir(filepath.Join(tmpDir, ".git"), 0755)

	cfg := SessionConfig{
		IsMock:      false,
		SessionName: "agent-fail-test",
		ProjectPath: tmpDir,
		Provider:    "openai", // triggers GetAgentClient
	}

	err := RunWorkflow(context.Background(), cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to initialize agent")
}

func TestProcessJiraTicket_SetupWorkspaceError(t *testing.T) {
	// Mock SetupWorkspace to fail
	originalSetup := cmdutils.SetupWorkspace
	defer func() { cmdutils.SetupWorkspace = originalSetup }()
	cmdutils.SetupWorkspace = func(ctx context.Context, gitClient git.IClient, repoURL, workspace, ticketID, epicKey, timestamp string) (string, error) {
		return "", assert.AnError
	}

	// Mock Jira
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	mux.HandleFunc("/rest/api/3/issue/SETUP-FAIL-1", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"key": "SETUP-FAIL-1",
			"fields": map[string]interface{}{
				"summary": "Setup Fail",
				"description": map[string]interface{}{
					"type": "doc", "version": 1,
					"content": []map[string]interface{}{
						{"type": "paragraph", "content": []map[string]interface{}{{"type": "text", "text": "Repo: https://github.com/example/repo"}}},
					},
				},
			},
		})
	})

	jClient := jira.NewClient(server.URL, "user", "token")
	cfg := SessionConfig{IsMock: true}

	err := ProcessJiraTicket(context.Background(), "SETUP-FAIL-1", jClient, cfg, nil)
	assert.Error(t, err)
	assert.Equal(t, assert.AnError, err)
}
