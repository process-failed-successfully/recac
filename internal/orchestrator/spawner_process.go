package orchestrator

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"recac/internal/git"
	"recac/internal/runner"
)

type ProcessSpawner struct {
	AgentBinary    string
	Poller         Poller
	AgentProvider  string
	AgentModel     string
	Logger         *slog.Logger
	SessionManager ISessionManager
	GitClient      IGitClient
	CmdFactory     func(name string, arg ...string) *exec.Cmd
}

func NewProcessSpawner(logger *slog.Logger, agentBinary string, poller Poller, provider, model string, sm ISessionManager) *ProcessSpawner {
	return &ProcessSpawner{
		AgentBinary:    agentBinary,
		Poller:         poller,
		AgentProvider:  provider,
		AgentModel:     model,
		Logger:         logger,
		SessionManager: sm,
		GitClient:      git.NewClient(),
		CmdFactory:     exec.Command,
	}
}

func (s *ProcessSpawner) Spawn(ctx context.Context, item WorkItem) error {
	// 1. Create temporary workspace
	tempDir, err := os.MkdirTemp("", fmt.Sprintf("recac-agent-%s-*", item.ID))
	if err != nil {
		return fmt.Errorf("failed to create temp workspace: %w", err)
	}

	s.Logger.Info("Spawning process agent for item", "id", item.ID, "workspace", tempDir)

	// 2. Prepare Command
	// recac-agent --jira ID --project ID --detached=false --cleanup=false --path /workspace --verbose --repo-url ...
	args := []string{
		"--jira", item.ID,
		"--project", item.ID,
		"--detached=false",
		"--cleanup=false",
		"--path", tempDir,
		"--verbose",
		"--repo-url", item.RepoURL,
	}

	cmd := s.CmdFactory(s.AgentBinary, args...)

	// 3. Prepare Environment
	env := os.Environ()
	env = append(env, fmt.Sprintf("RECAC_PROVIDER=%s", s.AgentProvider))
	env = append(env, fmt.Sprintf("RECAC_MODEL=%s", s.AgentModel))
	env = append(env, "GIT_TERMINAL_PROMPT=0")
	env = append(env, fmt.Sprintf("RECAC_PROJECT_ID=%s", item.ID))

	// Inject Git Identity
	env = append(env, "GIT_AUTHOR_NAME=RECAC Agent")
	env = append(env, "GIT_AUTHOR_EMAIL=agent@recac.io")
	env = append(env, "GIT_COMMITTER_NAME=RECAC Agent")
	env = append(env, "GIT_COMMITTER_EMAIL=agent@recac.io")

	// Secrets and Configs
	secrets := []string{"JIRA_API_TOKEN", "JIRA_USERNAME", "JIRA_URL", "GITHUB_TOKEN", "GITHUB_API_KEY", "OPENAI_API_KEY", "ANTHROPIC_API_KEY", "GEMINI_API_KEY", "OPENROUTER_API_KEY", "RECAC_DB_TYPE", "RECAC_DB_URL"}
	for _, secret := range secrets {
		if val := os.Getenv(secret); val != "" {
			env = append(env, fmt.Sprintf("%s=%s", secret, val))
			if secret == "GITHUB_API_KEY" {
				env = append(env, fmt.Sprintf("RECAC_GITHUB_API_KEY=%s", val))
			}
		}
	}

	// Notifications
	if val := os.Getenv("RECAC_NOTIFICATIONS_DISCORD_ENABLED"); val != "" {
		env = append(env, fmt.Sprintf("RECAC_NOTIFICATIONS_DISCORD_ENABLED=%s", val))
	}
	if val := os.Getenv("RECAC_NOTIFICATIONS_SLACK_ENABLED"); val != "" {
		env = append(env, fmt.Sprintf("RECAC_NOTIFICATIONS_SLACK_ENABLED=%s", val))
	}

	// Host Workspace Path (Same as path since we are on host)
	env = append(env, fmt.Sprintf("RECAC_HOST_WORKSPACE_PATH=%s", tempDir))

	// Agent limits
	if val := os.Getenv("RECAC_MAX_ITERATIONS"); val != "" {
		env = append(env, fmt.Sprintf("RECAC_MAX_ITERATIONS=%s", val))
	}
	if val := os.Getenv("RECAC_MANAGER_FREQUENCY"); val != "" {
		env = append(env, fmt.Sprintf("RECAC_MANAGER_FREQUENCY=%s", val))
	}
	if val := os.Getenv("RECAC_TASK_MAX_ITERATIONS"); val != "" {
		env = append(env, fmt.Sprintf("RECAC_TASK_MAX_ITERATIONS=%s", val))
	}

	// Item specific env
	for k, v := range item.EnvVars {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}

	cmd.Env = env

	logFile, err := os.Create(filepath.Join(tempDir, "agent.log"))
	if err == nil {
		cmd.Stdout = logFile
		cmd.Stderr = logFile
	} else {
		s.Logger.Warn("Failed to create log file", "error", err)
	}

	// 4. Start Process
	if err := cmd.Start(); err != nil {
		os.RemoveAll(tempDir)
		return fmt.Errorf("failed to start agent process: %w", err)
	}

	pid := cmd.Process.Pid

	// 5. Create Session
	session := &runner.SessionState{
		Name:           item.ID,
		StartTime:      time.Now(),
		Command:        args, // Just args for display
		Workspace:      tempDir,
		Status:         "running",
		Type:           "orchestrated-process",
		AgentStateFile: filepath.Join(tempDir, ".agent_state.json"),
		ContainerID:    fmt.Sprintf("pid-%d", pid), // Use PID as ID
	}

	if err := s.SessionManager.SaveSession(session); err != nil {
		s.Logger.Error("failed to save session state", "pid", pid, "error", err)
		// Try to kill
		cmd.Process.Kill()
		os.RemoveAll(tempDir)
		return err
	}

	s.Logger.Info("Agent process started", "pid", pid, "work_item", item.ID)

	// 6. Monitor in Background
	go func() {
		defer logFile.Close()

		err := cmd.Wait()

		// Reload session for final update
		finalSession, loadErr := s.SessionManager.LoadSession(item.ID)
		if loadErr != nil {
			s.Logger.Error("failed to load session for final update", "session", item.ID, "error", loadErr)
			if err != nil {
				_ = s.Poller.UpdateStatus(context.Background(), item, "Failed", fmt.Sprintf("Agent process failed: %v", err))
			}
			return
		}

		finalSession.EndTime = time.Now()
		if err != nil {
			finalSession.Status = "error"
			finalSession.Error = err.Error()
			s.Logger.Error("Agent process failed", "pid", pid, "error", err)
			_ = s.Poller.UpdateStatus(context.Background(), item, "Failed", fmt.Sprintf("Agent process failed: %v", err))
		} else {
			finalSession.Status = "completed"
			s.Logger.Info("Agent process completed", "pid", pid)
		}

		// Get end commit SHA
		endSHA, shaErr := s.GitClient.CurrentCommitSHA(tempDir)
		if shaErr != nil {
			s.Logger.Warn("could not get end commit SHA", "workspace", tempDir, "error", shaErr)
		} else {
			finalSession.EndCommitSHA = endSHA
		}

		if saveErr := s.SessionManager.SaveSession(finalSession); saveErr != nil {
			s.Logger.Error("failed to save final session state", "session", item.ID, "error", saveErr)
		}

		// Clean up workspace
		if removeErr := os.RemoveAll(tempDir); removeErr != nil {
			s.Logger.Warn("failed to clean up workspace", "path", tempDir, "error", removeErr)
		}
	}()

	return nil
}

func (s *ProcessSpawner) Cleanup(ctx context.Context, item WorkItem) error {
	return nil
}
