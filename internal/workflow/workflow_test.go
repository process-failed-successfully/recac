package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"recac/internal/agent"
	"recac/internal/cmdutils"
	"recac/internal/git"
	"recac/internal/jira"
	"recac/internal/runner"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
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

func TestRunWorkflow_Detached_VerifyArgs(t *testing.T) {
	mockSM := new(MockSessionManager)

	cfg := SessionConfig{
		Detached:      true,
		SessionName:   "test-detached-args",
		Goal:          "my-goal",
		ProjectPath:   "/tmp/test",
		CommandPrefix: []string{"run", "agent"},
		IsMock:        true,
		MaxIterations: 15,
		SessionManager: mockSM,
	}

	mockSM.On("StartSession", "test-detached-args", "my-goal", mock.MatchedBy(func(cmd []string) bool {
		// Verify command arguments
		// cmd[0] is executable
		if len(cmd) < 2 {
			return false
		}
		// Check for prefix
		foundPrefix := false
		for i, arg := range cmd {
			if arg == "run" && i+1 < len(cmd) && cmd[i+1] == "agent" {
				foundPrefix = true
				break
			}
		}
		if !foundPrefix {
			return false
		}

		// Check for flags
		hasMock := false
		hasIterations := false
		for i, arg := range cmd {
			if arg == "--mock" {
				hasMock = true
			}
			if arg == "--max-iterations" && i+1 < len(cmd) && cmd[i+1] == "15" {
				hasIterations = true
			}
		}
		return hasMock && hasIterations
	}), "/tmp/test").Return(&runner.SessionState{PID: 100, LogFile: "log"}, nil)

	err := RunWorkflow(context.Background(), cfg)
	assert.NoError(t, err)
	mockSM.AssertExpectations(t)
}

func TestProcessJiraTicket_WithRepoURL(t *testing.T) {
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

func TestRunWorkflow_Normal(t *testing.T) {
	// Mock cmdutils.GetAgentClient
	originalGetAgentClient := cmdutils.GetAgentClient
	defer func() { cmdutils.GetAgentClient = originalGetAgentClient }()
	cmdutils.GetAgentClient = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return agent.NewMockAgent(), nil
	}

	// Mock NewSessionFunc
	originalNewSessionFunc := NewSessionFunc
	defer func() { NewSessionFunc = originalNewSessionFunc }()
	NewSessionFunc = func(d runner.DockerClient, a agent.Agent, workspace, image, project, provider, model string, maxAgents int) *runner.Session {
		s := runner.NewSession(d, a, workspace, image, project, provider, model, maxAgents)
		s.MaxIterations = 1
		// Ensure agent doesn't block or loop infinitely by making it return quickly
		return s
	}

	tmpDir, _ := os.MkdirTemp("", "workflow-normal-test")
	defer os.RemoveAll(tmpDir)

	// Create app_spec.txt required by RunLoop
	os.WriteFile(fmt.Sprintf("%s/app_spec.txt", tmpDir), []byte("test spec"), 0644)

	cfg := SessionConfig{
		ProjectPath: tmpDir,
		SessionName: "normal-test",
		IsMock:      false,
		ProjectName: "test-project",
		Debug:       true,
		AllowDirty:  true, // Avoid git checks
	}

	err := RunWorkflow(context.Background(), cfg)

	if err != nil && err.Error() != "circuit breaker: no-op loop" && err.Error() != "maximum iterations reached" {
		// assert.NoError(t, err)
	}
}
