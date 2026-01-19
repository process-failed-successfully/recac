package orchestrator

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"recac/internal/git"
	"recac/internal/runner"
	"strings"
	"time"
)

type DockerSpawner struct {
	Client         DockerClient
	Image          string
	Network        string
	Poller         Poller // To update status on completion
	AgentProvider  string
	AgentModel     string
	projectName    string
	Logger         *slog.Logger
	SessionManager ISessionManager
	GitClient      IGitClient
}

func NewDockerSpawner(logger *slog.Logger, client DockerClient, image string, projectName string, poller Poller, provider, model string, sm ISessionManager) *DockerSpawner {
	return &DockerSpawner{
		Client:         client,
		Image:          image,
		projectName:    projectName,
		Poller:         poller,
		AgentProvider:  provider,
		AgentModel:     model,
		Logger:         logger,
		SessionManager: sm,
		GitClient:      git.NewClient(),
	}
}

func (s *DockerSpawner) Spawn(ctx context.Context, item WorkItem) error {
	// 1. Create temporary workspace on host
	tempDir, err := os.MkdirTemp("", fmt.Sprintf("recac-agent-%s-*", item.ID))
	if err != nil {
		return fmt.Errorf("failed to create temp workspace: %w", err)
	}

	// 2. Prepare workspace mounts
	// We no longer clone here (Host). We delegate cloning to the Agent (Container).
	// This ensures consistency with K8s and reduces host dependency.

	// Mounts
	binds := []string{
		fmt.Sprintf("%s:/workspace", tempDir),
		"/var/run/docker.sock:/var/run/docker.sock", // Enable DinD for agent
	}

	s.Logger.Info("Spawning agent for item", "id", item.ID, "workspace", tempDir)

	user := ""
	extraBinds := binds[1:] // only docker sock

	containerID, err := s.Client.RunContainer(ctx, s.Image, tempDir, extraBinds, nil, user)
	if err != nil {
		os.RemoveAll(tempDir)
		return fmt.Errorf("failed to start container: %w", err)
	}

	// 4. Create and save the initial session state
	agentCmd := []string{
		"/usr/local/bin/recac-agent",
		"--jira", item.ID,
		"--project", item.ID,
		"--detached=false",
		"--cleanup=false",
		"--path", "/workspace",
		"--verbose",
		"--repo-url", item.RepoURL, // Delegate cloning
	}

	session := &runner.SessionState{
		Name:           item.ID,
		StartTime:      time.Now(),
		Command:        agentCmd,
		Workspace:      tempDir,
		Status:         "running",
		Type:           "orchestrated-docker",
		AgentStateFile: filepath.Join(tempDir, ".agent_state.json"),
		StartCommitSHA: "", // Unknown at start, populated at end
	}

	if err := s.SessionManager.SaveSession(session); err != nil {
		s.Logger.Error("failed to save session, cleaning up container", "container", containerID, "error", err)
		if stopErr := s.Client.StopContainer(context.Background(), containerID); stopErr != nil {
			s.Logger.Warn("failed to stop container during cleanup", "container", containerID, "error", stopErr)
		}
		os.RemoveAll(tempDir)
		return fmt.Errorf("failed to save session state: %w", err)
	}

	s.Logger.Info("Container started", "id", containerID, "work_item", item.ID)

	// 5. Execute Work in Background
	go func() {
		// Construct Command
		var envExports []string
		if s.AgentProvider != "" {
			envExports = append(envExports, fmt.Sprintf("export RECAC_PROVIDER='%s'", s.AgentProvider))
		}
		if s.AgentModel != "" {
			envExports = append(envExports, fmt.Sprintf("export RECAC_MODEL='%s'", s.AgentModel))
		}
		envExports = append(envExports, "export GIT_TERMINAL_PROMPT=0")
		envExports = append(envExports, fmt.Sprintf("export RECAC_PROJECT_ID='%s'", item.ID))

		// Inject Git Identity to prevent "Author identity unknown" errors
		envExports = append(envExports, "export GIT_AUTHOR_NAME='RECAC Agent'")
		envExports = append(envExports, "export GIT_AUTHOR_EMAIL='agent@recac.io'")
		envExports = append(envExports, "export GIT_COMMITTER_NAME='RECAC Agent'")
		envExports = append(envExports, "export GIT_COMMITTER_EMAIL='agent@recac.io'")

		// Propagate Notifications Config
		if val := os.Getenv("RECAC_NOTIFICATIONS_DISCORD_ENABLED"); val != "" {
			envExports = append(envExports, fmt.Sprintf("export RECAC_NOTIFICATIONS_DISCORD_ENABLED='%s'", val))
		}
		if val := os.Getenv("RECAC_NOTIFICATIONS_SLACK_ENABLED"); val != "" {
			envExports = append(envExports, fmt.Sprintf("export RECAC_NOTIFICATIONS_SLACK_ENABLED='%s'", val))
		}

		for k, v := range item.EnvVars {
			envExports = append(envExports, fmt.Sprintf("export %s='%s'", k, v))
		}

		secrets := []string{"JIRA_API_TOKEN", "JIRA_USERNAME", "JIRA_URL", "GITHUB_TOKEN", "GITHUB_API_KEY", "OPENAI_API_KEY", "ANTHROPIC_API_KEY", "GEMINI_API_KEY", "OPENROUTER_API_KEY", "RECAC_DB_TYPE", "RECAC_DB_URL"}
		for _, secret := range secrets {
			if val := os.Getenv(secret); val != "" {
				envExports = append(envExports, fmt.Sprintf("export %s='%s'", secret, val))
				if secret == "GITHUB_API_KEY" {
					envExports = append(envExports, fmt.Sprintf("export RECAC_GITHUB_API_KEY='%s'", val))
				}
			}
		}

		envExports = append(envExports, fmt.Sprintf("export RECAC_HOST_WORKSPACE_PATH='%s'", tempDir))

		cmdStr := "cd /workspace"
		cmdStr += " && export RECAC_MAX_ITERATIONS=20"
		cmdStr += " && " + strings.Join(envExports, " && ")
		cmdStr += " && " + strings.Join(agentCmd, " ")
		cmdStr += " && echo 'Recac Finished'"

		cmd := []string{"/bin/sh", "-c", cmdStr}

		s.Logger.Info("Executing agent command", "item", item.ID)
		output, execErr := s.Client.Exec(context.Background(), containerID, cmd)

		// 6. Update session state
		finalSession, loadErr := s.SessionManager.LoadSession(item.ID)
		if loadErr != nil {
			s.Logger.Error("failed to load session for final update", "session", item.ID, "error", loadErr)
			// Still update poller status
			if execErr != nil {
				_ = s.Poller.UpdateStatus(context.Background(), item, "Failed", fmt.Sprintf("Agent failed:\n%s\nOutput:\n%s", execErr, output))
			}
			return
		}

		finalSession.EndTime = time.Now()
		if execErr != nil {
			finalSession.Status = "error"
			finalSession.Error = execErr.Error()
			s.Logger.Error("Agent execution failed", "item", item.ID, "error", execErr, "output", output)
			_ = s.Poller.UpdateStatus(context.Background(), item, "Failed", fmt.Sprintf("Agent failed:\n%s\nOutput:\n%s", execErr, output))
		} else {
			finalSession.Status = "completed"
			s.Logger.Info("Agent execution completed", "item", item.ID, "output", string(output))
		}

		// 7. Get end commit SHA
		endSHA, shaErr := s.GitClient.CurrentCommitSHA(tempDir)
		if shaErr != nil {
			s.Logger.Warn("could not get end commit SHA", "workspace", tempDir, "error", shaErr)
		} else {
			finalSession.EndCommitSHA = endSHA
		}

		if err := s.SessionManager.SaveSession(finalSession); err != nil {
			s.Logger.Error("failed to save final session state", "session", item.ID, "error", err)
		}

		// 8. Clean up workspace
		if err := os.RemoveAll(tempDir); err != nil {
			s.Logger.Warn("failed to clean up workspace", "path", tempDir, "error", err)
		}
	}()

	return nil
}

func (s *DockerSpawner) Cleanup(ctx context.Context, item WorkItem) error {
	// For now, we rely on the agent's own cleanup and don't manage the container lifecycle here.
	// Future implementation could stop/remove the container.
	return nil
}
