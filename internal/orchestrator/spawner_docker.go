package orchestrator

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
)

// dockerClient defines the interface for Docker operations that the spawner needs.
// This allows for mocking in tests.
type dockerClient interface {
	RunContainer(ctx context.Context, image, workspace string, extraBinds []string, env []string, user string) (string, error)
	Exec(ctx context.Context, containerID string, cmd []string) (string, error)
}

type DockerSpawner struct {
	Client        dockerClient
	Image         string
	Network       string
	Poller        Poller // To update status on completion
	AgentProvider string
	AgentModel    string
	projectName   string
	Logger        *slog.Logger
}

func NewDockerSpawner(logger *slog.Logger, client dockerClient, image string, projectName string, poller Poller, provider, model string) *DockerSpawner {
	return &DockerSpawner{
		Client:        client,
		Image:         image,
		projectName:   projectName,
		Poller:        poller,
		AgentProvider: provider,
		AgentModel:    model,
		Logger:        logger,
	}
}

func (s *DockerSpawner) Spawn(ctx context.Context, item WorkItem) error {
	// 1. Create temporary workspace on host
	tempDir, err := os.MkdirTemp("", fmt.Sprintf("recac-agent-%s-*", item.ID))
	if err != nil {
		return fmt.Errorf("failed to create temp workspace: %w", err)
	}

	s.Logger.Info("Spawning agent for item", "id", item.ID, "workspace", tempDir)

	// 2. Start Container
	extraBinds := []string{"/var/run/docker.sock:/var/run/docker.sock"}
	containerID, err := s.Client.RunContainer(ctx, s.Image, tempDir, extraBinds, nil, "")
	if err != nil {
		os.RemoveAll(tempDir) // Clean up on failure
		return fmt.Errorf("failed to start container: %w", err)
	}

	s.Logger.Info("Container started", "id", containerID, "work_item", item.ID)

	// 3. Execute Work in Background
	go func() {
		var envExports []string
		if s.AgentProvider != "" {
			envExports = append(envExports, fmt.Sprintf("export RECAC_PROVIDER='%s'", s.AgentProvider))
		}
		if s.AgentModel != "" {
			envExports = append(envExports, fmt.Sprintf("export RECAC_MODEL='%s'", s.AgentModel))
		}
		envExports = append(envExports, "export GIT_TERMINAL_PROMPT=0")

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

		authRepoURL := item.RepoURL
		ghToken := os.Getenv("GITHUB_TOKEN")
		if ghToken == "" {
			ghToken = os.Getenv("GITHUB_API_KEY")
		}
		if ghToken != "" && strings.Contains(authRepoURL, "github.com") && !strings.Contains(authRepoURL, "@") {
			ghToken = strings.Trim(ghToken, "\"")
			authRepoURL = strings.Replace(authRepoURL, "https://github.com/", fmt.Sprintf("https://%s@github.com/", ghToken), 1)
		}

		var cmdStr string
		cmdStr = "cd /workspace" // Reset to constant
		cmdStr += " && export RECAC_MAX_ITERATIONS=20"
		cmdStr += " && " + strings.Join(envExports, " && ")
		if authRepoURL != "" {
			cmdStr += fmt.Sprintf(" && git clone --depth 1 %s .", authRepoURL)
		}
		cmdStr += fmt.Sprintf(" && /usr/local/bin/recac start --jira %s --project %s --detached=false --cleanup=false --path /workspace --verbose", item.ID, item.ID)
		cmdStr += " && echo 'Recac Finished'"

		cmd := []string{"/bin/sh", "-c", cmdStr}

		s.Logger.Info("Executing agent command", "item", item.ID)
		output, execErr := s.Client.Exec(context.Background(), containerID, cmd)

		if execErr != nil {
			s.Logger.Error("Agent execution failed", "item", item.ID, "error", execErr, "output", output)
			if s.Poller != nil {
				_ = s.Poller.UpdateStatus(context.Background(), item, "Failed", fmt.Sprintf("Agent failed:\n%s\nOutput:\n%s", execErr, output))
			}
		} else {
			s.Logger.Info("Agent execution completed", "item", item.ID, "output", output)
		}
	}()

	return nil
}

func (s *DockerSpawner) Cleanup(ctx context.Context, item WorkItem) error {
	// Not implemented for Docker, containers are ephemeral
	return nil
}
