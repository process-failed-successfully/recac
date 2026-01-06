package orchestrator

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"recac/internal/docker"
	"strings"
)

type DockerSpawner struct {
	Client        *docker.Client
	Image         string
	Network       string
	Poller        Poller // To update status on completion
	AgentProvider string
	AgentModel    string
	projectName   string
	Logger        *slog.Logger
}

func NewDockerSpawner(logger *slog.Logger, client *docker.Client, image string, projectName string, poller Poller, provider, model string) *DockerSpawner {
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
	// Using os.MkdirTemp ensures unique directory per agent execution
	// Start in current directory (or TMPDIR)
	tempDir, err := os.MkdirTemp("", fmt.Sprintf("recac-agent-%s-*", item.ID))
	if err != nil {
		return fmt.Errorf("failed to create temp workspace: %w", err)
	}

	// Mounts
	binds := []string{
		fmt.Sprintf("%s:/workspace", tempDir),
		"/var/run/docker.sock:/var/run/docker.sock", // Enable DinD for agent
	}

	s.Logger.Info("Spawning agent for item", "id", item.ID, "workspace", tempDir)

	// 2. Start Container (Long-running)
	// We use the item ID as part of container name? Not exposed in RunContainer.
	// We'll trust RunContainer to return ID.
	// Run as root to ensure access to /var/run/docker.sock (DinD requirement)
	// and to avoid permission issues with /root/.config/git
	// user := "0:0"
	user := ""

	// 2. Run Container
	// We run it as key 'recac' (EntryPoint) is defined in image, but we want to run shell logic to setup env first?
	// ACTUALLY the internal/docker client RunContainer handles some of this.
	// But we need to pass env vars.
	// We can't easily pass Env via RunContainer args if it only takes []string.
	// Wait, RunContainer signature: (ctx, image, workspace, extraBinds, user)
	// It doesn't take Env vars?
	// internal/docker/client.go: RunContainer(...)
	// It does NOT take Env list independently!
	// It hardcodes logic.
	// BUT, we are using `Exec` to run the command.
	// So we export Env inside the `sh -c` command string.
	// So passing Env to RunContainer is not needed for the shell, but might be good for cleanliness.
	// However, `docker.Client` uses `extraBinds`.
	// So we pass our `binds` slice (excluding workspace which is arg 2).

	// Remove workspace from binds because RunContainer adds it
	extraBinds := binds[1:] // only docker sock

	containerID, err := s.Client.RunContainer(ctx, s.Image, tempDir, extraBinds, nil, user)
	if err != nil {
		os.RemoveAll(tempDir)
		return fmt.Errorf("failed to start container: %w", err)
	}

	s.Logger.Info("Container started", "id", containerID, "work_item", item.ID)

	// 3. Execute Work in Background
	go func() {
		/* defer func() {
			// Cleanup container
			_ = s.Client.StopContainer(context.Background(), containerID)
			// Cleanup workspace? Spec says "Persistence: Any changes must be pushed...".
			// "Ephemeral: The workspace exists only for the duration of the Job".
			// So yes, cleanup.
			_ = os.RemoveAll(tempDir)
		}() */

		// Construct Command
		// 1. Export Env Vars
		// 2. Clone Repo (Auth?)
		// 3. Run recac

		// Inject standard environment variables
		var envExports []string
		if s.AgentProvider != "" {
			envExports = append(envExports, fmt.Sprintf("export RECAC_PROVIDER='%s'", s.AgentProvider))
		}
		if s.AgentModel != "" {
			envExports = append(envExports, fmt.Sprintf("export RECAC_MODEL='%s'", s.AgentModel))
		}
		envExports = append(envExports, "export GIT_TERMINAL_PROMPT=0") // Prevent git from prompting for credentials (hangs)

		for k, v := range item.EnvVars {
			envExports = append(envExports, fmt.Sprintf("export %s='%s'", k, v))
		}

		// Pass through host secrets if available in env?
		// We should probably explicitly pass keys if they are in the 'item.EnvVars' or global config.
		// For now assume Poller populated EnvVars with necessary keys (like JIRA_TOKEN).
		// But Poller doesn't know secrets.
		// The Orchestrator or Spawner should inject secrets.
		// We'll inject secrets here from HOST env.
		secrets := []string{"JIRA_API_TOKEN", "JIRA_USERNAME", "JIRA_URL", "GITHUB_TOKEN", "GITHUB_API_KEY", "OPENAI_API_KEY", "ANTHROPIC_API_KEY", "GEMINI_API_KEY", "OPENROUTER_API_KEY"}
		for _, secret := range secrets {
			if val := os.Getenv(secret); val != "" {
				envExports = append(envExports, fmt.Sprintf("export %s='%s'", secret, val))
				// Also map GITHUB_API_KEY to RECAC_GITHUB_API_KEY for Viper compatibility
				if secret == "GITHUB_API_KEY" {
					envExports = append(envExports, fmt.Sprintf("export RECAC_GITHUB_API_KEY='%s'", val))
				}
			}
		}

		// PROPAGATE HOST WORKSPACE PATH: For DinD volume mounting support
		envExports = append(envExports, fmt.Sprintf("export RECAC_HOST_WORKSPACE_PATH='%s'", tempDir))

		authRepoURL := item.RepoURL
		ghToken := os.Getenv("GITHUB_TOKEN")
		if ghToken == "" {
			ghToken = os.Getenv("GITHUB_API_KEY")
		}
		if ghToken != "" && strings.Contains(authRepoURL, "github.com") {
			// Sanitize token just in case
			ghToken = strings.Trim(ghToken, "\"")
			authRepoURL = strings.Replace(authRepoURL, "https://github.com/", fmt.Sprintf("https://%s@github.com/", ghToken), 1)
		}

		// Command Chain
		// git clone <url> .
		// recac start --jira <ID> --detached=false --cleanup=false (since we cleanup container)
		// We assume /workspace is empty initially.

		cmdStr := fmt.Sprintf("cd %s", tempDir) // Use the mounted path inside container? No, inside container it is /workspace!
		// Wait, binds are host_path:/workspace.
		// So inside container we must use /workspace.
		cmdStr = "cd /workspace" // Reset to constant
		cmdStr += " && export RECAC_MAX_ITERATIONS=10"
		cmdStr += " && " + strings.Join(envExports, " && ")
		cmdStr += fmt.Sprintf(" && /usr/local/bin/recac start --jira %s --project %s-%s --detached=false --cleanup=false --path /workspace --verbose", item.ID, s.projectName, item.ID)
		cmdStr += " && echo 'Recac Finished'"

		// Run via sh -c
		cmd := []string{"/bin/sh", "-c", cmdStr}

		// We use a detached context for the execution so it doesn't die if main context is cancelled immediately (unless we want that).
		// But Spawn passes 'ctx'. If 'ctx' is long-lived Orchestrator context, it's fine.
		// If 'ctx' is poll timeout, we shouldn't use it.
		// Orchestrator loop passes the main context.

		s.Logger.Info("Executing agent command", "item", item.ID)
		output, err := s.Client.Exec(context.Background(), containerID, cmd)

		if err != nil {
			s.Logger.Error("Agent execution failed", "item", item.ID, "error", err, "output", output)
			_ = s.Poller.UpdateStatus(context.Background(), item, "Failed", fmt.Sprintf("Agent failed:\n%s\nOutput:\n%s", err, output))
		} else {
			s.Logger.Info("Agent execution completed", "item", item.ID, "output", string(output))
		}
	}()

	return nil
}

func (s *DockerSpawner) Cleanup(ctx context.Context, item WorkItem) error {
	// Cleanup is handled in the background goroutine
	return nil
}
