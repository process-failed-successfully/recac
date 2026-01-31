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

	"github.com/stretchr/testify/assert"
)

func TestProcessDirectTask_Errors(t *testing.T) {
	// Mock SetupWorkspace
	originalSetup := cmdutils.SetupWorkspace
	defer func() { cmdutils.SetupWorkspace = originalSetup }()

	t.Run("SetupWorkspace Failure", func(t *testing.T) {
		cmdutils.SetupWorkspace = func(ctx context.Context, gitClient git.IClient, repoURL, workspace, ticketID, epicKey, timestamp string) (string, error) {
			return "", errors.New("setup failed")
		}

		cfg := SessionConfig{
			ProjectPath: "/tmp/whatever",
			IsMock:      true,
		}

		err := ProcessDirectTask(context.Background(), cfg)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "setup failed")
	})

	t.Run("WriteFile Failure (app_spec.txt)", func(t *testing.T) {
		cmdutils.SetupWorkspace = func(ctx context.Context, gitClient git.IClient, repoURL, workspace, ticketID, epicKey, timestamp string) (string, error) {
			return repoURL, nil
		}

		// Create a file to block usage as directory
		tmpFile, _ := os.CreateTemp("", "workflow-blocker")
		defer os.Remove(tmpFile.Name())

		// We pass the file path as ProjectPath.
		// SetupWorkspace mocks success, but then writing app_spec.txt inside it should fail if we force it to treat it as dir?
		// ProcessDirectTask does: specPath := filepath.Join(cfg.ProjectPath, "app_spec.txt")
		// If ProjectPath is a file, filepath.Join works, but os.WriteFile will fail with ENOTDIR.

		cfg := SessionConfig{
			ProjectPath: tmpFile.Name(),
			Summary:     "Test Summary", // Trigger app_spec write
			IsMock:      true,
		}

		err := ProcessDirectTask(context.Background(), cfg)
		assert.Error(t, err)
		// Error message depends on OS, usually "not a directory"
	})
}

func TestRunWorkflow_Errors(t *testing.T) {
	t.Run("Dirty Git State", func(t *testing.T) {
		// Setup a real git repo
		tmpDir, _ := os.MkdirTemp("", "workflow-dirty-test")
		defer os.RemoveAll(tmpDir)

		// Initialize git
		exec.Command("git", "init", tmpDir).Run()

		// Configure git user
		exec.Command("git", "-C", tmpDir, "config", "user.email", "test@example.com").Run()
		exec.Command("git", "-C", tmpDir, "config", "user.name", "Test User").Run()

		// Create a file and commit it
		os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte("initial"), 0644)
		exec.Command("git", "-C", tmpDir, "add", ".").Run()
		exec.Command("git", "-C", tmpDir, "commit", "-m", "initial").Run()

		// Make it dirty
		os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte("modified"), 0644)

		cfg := SessionConfig{
			ProjectPath: tmpDir,
			AllowDirty:  false,
			SessionName: "dirty-test",
		}

		err := RunWorkflow(context.Background(), cfg)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "uncommitted changes detected")
	})

	t.Run("Agent Client Failure", func(t *testing.T) {
		originalGetAgentClient := cmdutils.GetAgentClient
		defer func() { cmdutils.GetAgentClient = originalGetAgentClient }()

		cmdutils.GetAgentClient = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
			return nil, errors.New("agent init failed")
		}

		tmpDir, _ := os.MkdirTemp("", "workflow-agent-fail")
		defer os.RemoveAll(tmpDir)

		cfg := SessionConfig{
			ProjectPath: tmpDir,
			AllowDirty:  true, // Skip git check
			SessionName: "agent-fail-test",
		}

		err := RunWorkflow(context.Background(), cfg)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to initialize agent")
	})
}

func TestProcessJiraTicket_Errors(t *testing.T) {
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
					"type": "doc",
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

	jClient := jira.NewClient(server.URL, "user", "token")

	t.Run("Workspace Creation Failure", func(t *testing.T) {
		// Create a file to block directory creation
		tmpFile, _ := os.CreateTemp("", "workflow-jira-blocker")
		defer os.Remove(tmpFile.Name())

		// ProcessJiraTicket does: if cfg.ProjectPath != "" { ... os.MkdirAll(tempWorkspace, 0755) ... }
		// Pass the file path as ProjectPath
		cfg := SessionConfig{
			ProjectPath: tmpFile.Name(),
			IsMock:      true,
		}

		err := ProcessJiraTicket(context.Background(), "TEST-1", jClient, cfg, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not a directory") // or similar OS error
	})

	t.Run("SetupWorkspace Failure", func(t *testing.T) {
		originalSetup := cmdutils.SetupWorkspace
		defer func() { cmdutils.SetupWorkspace = originalSetup }()

		cmdutils.SetupWorkspace = func(ctx context.Context, gitClient git.IClient, repoURL, workspace, ticketID, epicKey, timestamp string) (string, error) {
			return "", errors.New("setup failed")
		}

		tmpDir, _ := os.MkdirTemp("", "workflow-jira-setup-fail")
		defer os.RemoveAll(tmpDir)

		cfg := SessionConfig{
			ProjectPath: tmpDir,
			IsMock:      true,
		}

		err := ProcessJiraTicket(context.Background(), "TEST-1", jClient, cfg, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "setup failed")
	})

	t.Run("WriteFile Failure (app_spec.txt)", func(t *testing.T) {
		originalSetup := cmdutils.SetupWorkspace
		defer func() { cmdutils.SetupWorkspace = originalSetup }()

		tmpDir, _ := os.MkdirTemp("", "workflow-jira-write-fail")
		defer os.RemoveAll(tmpDir)

		// Make it read-only
		os.Chmod(tmpDir, 0555)

		cfg := SessionConfig{
			ProjectPath: tmpDir,
			IsMock:      true,
		}

		// Mock SetupWorkspace to return the workspace correctly
		cmdutils.SetupWorkspace = func(ctx context.Context, gitClient git.IClient, repoURL, workspace, ticketID, epicKey, timestamp string) (string, error) {
			return workspace, nil
		}

		err := ProcessJiraTicket(context.Background(), "TEST-1", jClient, cfg, nil)

		// If root is read-only, os.WriteFile(specPath) should fail.
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "permission denied")

		// Cleanup: fix perm so defer RemoveAll works
		os.Chmod(tmpDir, 0755)
	})
}
