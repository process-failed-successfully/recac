package workflow

import (
	"context"
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

	"github.com/stretchr/testify/assert"
)

func runCmd(dir, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	return cmd.Run()
}

func TestProcessDirectTask_Errors(t *testing.T) {
	// Mock SetupWorkspace to fail
	originalSetup := cmdutils.SetupWorkspace
	defer func() { cmdutils.SetupWorkspace = originalSetup }()

	cmdutils.SetupWorkspace = func(ctx context.Context, gitClient git.IClient, repoURL, workspace, ticketID, epicKey, timestamp string) (string, error) {
		return "", errors.New("setup failed")
	}

	cfg := SessionConfig{
		ProjectPath: "/tmp/nonexistent", // Path doesn't matter as setup mocked
		RepoURL:     "https://github.com/example/direct",
		Summary:     "Do something",
		IsMock:      true,
	}

	err := ProcessDirectTask(context.Background(), cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "setup failed")

	// Mock writing failure (directory not writable)
	// We restore SetupWorkspace to success but use a bad path
	cmdutils.SetupWorkspace = func(ctx context.Context, gitClient git.IClient, repoURL, workspace, ticketID, epicKey, timestamp string) (string, error) {
		return repoURL, nil
	}

	// Create a read-only directory
	tmpDir, _ := os.MkdirTemp("", "workflow-direct-fail")
	defer os.RemoveAll(tmpDir)
	os.Chmod(tmpDir, 0555) // Read-only

	cfg.ProjectPath = tmpDir
	// Note: We need to set IsMock=true to avoid actual git clone if real SetupWorkspace was called,
	// but here we mocked it.
	// However, ProcessDirectTask writes app_spec.txt AFTER SetupWorkspace.
	// So if we pass a dir that is read-only, os.WriteFile should fail.

	err = ProcessDirectTask(context.Background(), cfg)
	assert.Error(t, err)
	// Error might vary by OS, usually "permission denied"
	// assert.Contains(t, err.Error(), "permission denied")
}

func TestProcessJiraTicket_Errors(t *testing.T) {
	// 1. Ticket Fetch Error
	// Mock Jira Server to return 404
	mux := http.NewServeMux()
	mux.HandleFunc("/rest/api/3/issue/TEST-FAIL", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	jClient := jira.NewClient(server.URL, "user", "token")
	cfg := SessionConfig{IsMock: true}

	err := ProcessJiraTicket(context.Background(), "TEST-FAIL", jClient, cfg, nil)
	assert.Error(t, err)

	// 2. Blocked Ticket
	mux.HandleFunc("/rest/api/3/issue/TEST-BLOCKED", func(w http.ResponseWriter, r *http.Request) {
		// Return ticket with blockers (linked issues)
		// We need to mock how GetBlockers works.
		// It usually checks issuelinks.
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"key": "TEST-BLOCKED",
			"fields": {
				"summary": "Blocked Ticket",
				"issuelinks": [
					{
						"type": {"inward": "is blocked by"},
						"inwardIssue": {"key": "BLOCK-1", "fields": {"status": {"name": "Open"}}}
					}
				]
			}
		}`))
	})

	err = ProcessJiraTicket(context.Background(), "TEST-BLOCKED", jClient, cfg, map[string]bool{})
	assert.NoError(t, err) // Should return nil (skipped)

	// 3. No Repo URL
	mux.HandleFunc("/rest/api/3/issue/TEST-NOREPO", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"key": "TEST-NOREPO",
			"fields": {
				"summary": "No Repo Ticket",
				"description": {"type": "doc", "content": [{"type": "paragraph", "content": [{"type": "text", "text": "Just text"}]}]},
				"issuelinks": []
			}
		}`))
	})

	err = ProcessJiraTicket(context.Background(), "TEST-NOREPO", jClient, cfg, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no repo url found")

	// 4. Invalid Ticket Format (missing fields)
	mux.HandleFunc("/rest/api/3/issue/TEST-INVALID", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"key": "TEST-INVALID"}`)) // Missing "fields"
	})
	err = ProcessJiraTicket(context.Background(), "TEST-INVALID", jClient, cfg, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid ticket format")
}

func TestRunWorkflow_Errors(t *testing.T) {
	// 1. Detached Mode Errors
	cfg := SessionConfig{
		Detached:    true,
		SessionName: "", // Missing name
	}
	err := RunWorkflow(context.Background(), cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "--name is required")

	// Mock SessionManager
	originalNewSessionManager := NewSessionManagerFunc
	defer func() { NewSessionManagerFunc = originalNewSessionManager }()

	NewSessionManagerFunc = func() (ISessionManager, error) {
		return nil, errors.New("manager creation failed")
	}

	cfg.SessionName = "test-detached"
	// We need to pass check for executable, but we can rely on fallback to recac-app or fail earlier if executable lookup fails.
	// But getting executable path usually succeeds.

	// If we provide a non-existent path in session manager, it might fail.
	// But here we mocked NewSessionManagerFunc to fail.

	err = RunWorkflow(context.Background(), cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "manager creation failed")

	// 2. Normal Mode Dirty Check
	// Restore NewSessionManagerFunc
	NewSessionManagerFunc = originalNewSessionManager

	tmpDir, _ := os.MkdirTemp("", "workflow-dirty-test")
	defer os.RemoveAll(tmpDir)

	// Init git repo
	runCmd(tmpDir, "git", "init")
	runCmd(tmpDir, "git", "config", "user.email", "you@example.com")
	runCmd(tmpDir, "git", "config", "user.name", "Your Name")
	os.WriteFile(filepath.Join(tmpDir, "file.txt"), []byte("dirty"), 0644)
	runCmd(tmpDir, "git", "add", "file.txt")
	// Uncommitted changes

	cfg = SessionConfig{
		Detached:    false,
		ProjectPath: tmpDir,
		SessionName: "dirty-test",
		AllowDirty:  false,
		IsMock:      false, // Normal mode
	}

	err = RunWorkflow(context.Background(), cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "uncommitted changes detected")

	// 3. Agent Client Error
	// We skip AllowDirty
	cfg.AllowDirty = true

	// Mock cmdutils.GetAgentClient to fail
	originalGetAgent := cmdutils.GetAgentClient
	defer func() { cmdutils.GetAgentClient = originalGetAgent }()
	cmdutils.GetAgentClient = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return nil, errors.New("agent init failed")
	}

	err = RunWorkflow(context.Background(), cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "agent init failed")
}
