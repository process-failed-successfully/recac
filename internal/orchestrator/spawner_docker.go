package orchestrator

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"recac/internal/git"
	"recac/internal/runner"
	"sort"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/kballard/go-shellquote"
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

	// Label the container for Janitor
	labels := map[string]string{
		"created-by": "recac-orchestrator",
		"work-item":  item.ID,
		"created-at": time.Now().Format(time.RFC3339),
	}

	// 4. Prepare Environment and Command
	envMap := collectAgentEnvVars(item, s.AgentProvider, s.AgentModel)
	envMap["RECAC_HOST_WORKSPACE_PATH"] = tempDir
	var env []string
	for k, v := range envMap {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	sort.Strings(env)

	agentCmd := []string{
		"/usr/local/bin/recac-agent",
		"--jira", item.ID,
		"--project", item.ID,
		"--image", s.Image,
		"--detached=false",
		"--cleanup=false",
		"--path", "/workspace",
		"--verbose",
		"--repo-url", item.RepoURL, // Delegate cloning
	}

	cmd := s.constructShellCommand(agentCmd)

	// 5. Create and Start Container
	containerID, err := s.Client.RunContainerWithLabels(ctx, s.Image, tempDir, extraBinds, env, cmd, user, labels)
	if err != nil {
		os.RemoveAll(tempDir)
		return fmt.Errorf("failed to start container: %w", err)
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
		ContainerID:    containerID,
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

	// 6. Wait for Completion
	s.Logger.Info("Waiting for agent completion", "item", item.ID)
	exitCode, waitErr := s.Client.WaitContainer(ctx, containerID)

	var output string
	var execErr error
	if waitErr != nil {
		execErr = waitErr
	} else if exitCode != 0 {
		execErr = fmt.Errorf("agent exited with code %d", exitCode)
	}

	// If failed, try to get some logs
	if execErr != nil {
		logs, err := s.Client.ContainerLogs(ctx, containerID)
		if err == nil {
			// Read up to 4KB of logs for context
			buf := make([]byte, 4096)
			n, _ := logs.Read(buf)
			output = string(buf[:n])
			logs.Close()
		}
	}

	// 7. Update session state
	finalSession, loadErr := s.SessionManager.LoadSession(item.ID)
	if loadErr != nil {
		s.Logger.Error("failed to load session for final update", "session", item.ID, "error", loadErr)
		// Still update poller status
		if execErr != nil {
			_ = s.Poller.UpdateStatus(ctx, item, "Failed", fmt.Sprintf("Agent failed:\n%s\nOutput:\n%s", execErr, output))
		}
		return nil
	}

	finalSession.EndTime = time.Now()
	if execErr != nil {
		finalSession.Status = "error"
		finalSession.Error = execErr.Error()
		s.Logger.Error("Agent execution failed", "item", item.ID, "error", execErr, "output", output)
		_ = s.Poller.UpdateStatus(ctx, item, "Failed", fmt.Sprintf("Agent failed:\n%s\nOutput:\n%s", execErr, output))
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

	return nil
}

func (s *DockerSpawner) Cancel(ctx context.Context, jobID string) error {
	s.Logger.Info("Canceling job", "job", jobID)
	args := filters.NewArgs()
	args.Add("label", fmt.Sprintf("work-item=%s", jobID))
	// List only running containers
	containers, err := s.Client.ListContainers(ctx, container.ListOptions{Filters: args})
	if err != nil {
		return fmt.Errorf("failed to list containers: %w", err)
	}

	if len(containers) == 0 {
		return fmt.Errorf("no active container found for job %s", jobID)
	}

	for _, c := range containers {
		s.Logger.Info("Stopping container", "container", c.ID, "job", jobID)
		if err := s.Client.StopContainer(ctx, c.ID); err != nil {
			s.Logger.Warn("failed to stop container", "container", c.ID, "error", err)
			// Continue trying to stop others if any
		}
	}
	return nil
}

func (s *DockerSpawner) GetLogs(ctx context.Context, jobID string) (io.ReadCloser, error) {
	// Find container by label
	args := filters.NewArgs()
	args.Add("label", fmt.Sprintf("work-item=%s", jobID))
	containers, err := s.Client.ListContainers(ctx, container.ListOptions{Filters: args, All: true})
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}

	if len(containers) == 0 {
		return nil, fmt.Errorf("no active container found for job %s", jobID)
	}

	// Should be only one, but take the first one (most recent usually if multiple due to retries?)
	// ListContainers returns most recent first by default if not specified otherwise.
	containerID := containers[0].ID
	return s.Client.ContainerLogs(ctx, containerID)
}

func (s *DockerSpawner) constructShellCommand(agentCmd []string) []string {
	// Inject Git Config for GITHUB_TOKEN if present
	cmdStr := "if [ -n \"$GITHUB_TOKEN\" ]; then git config --global url.\"https://${GITHUB_TOKEN}:x-oauth-basic@github.com/\".insteadOf \"https://github.com/\"; fi"
	cmdStr += " && " + shellquote.Join(agentCmd...) + " --allow-dirty"
	cmdStr += " && echo 'Recac Finished'"

	return []string{"/bin/sh", "-c", cmdStr}
}

func (s *DockerSpawner) Cleanup(ctx context.Context, item WorkItem) error {
	// For now, we rely on the agent's own cleanup and don't manage the container lifecycle here.
	// Future implementation could stop/remove the container.
	return nil
}

func (s *DockerSpawner) Ping(ctx context.Context) error {
	// Check Docker daemon connectivity
	// We use ListContainers with Limit 1 as a lightweight ping
	_, err := s.Client.ListContainers(ctx, container.ListOptions{Limit: 1})
	if err != nil {
		return fmt.Errorf("docker daemon unreachable: %w", err)
	}
	return nil
}
