package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"recac/internal/cmdutils"
	"recac/internal/docker"
	"recac/internal/orchestrator"
	"recac/internal/runner"
	"recac/internal/telemetry"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	corev1 "k8s.io/api/core/v1"
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Start the orchestrator service",
	Run:   runOrchestrator,
}

func init() {
	rootCmd.AddCommand(runCmd)

	runCmd.Flags().String("mode", "local", "Orchestrator mode: 'local' (Docker) or 'k8s' (Kubernetes Job)")
	runCmd.Flags().String("jira-label", "recac-agent", "Jira label to poll for")
	runCmd.Flags().String("image", "ghcr.io/process-failed-successfully/recac-agent:latest", "Agent image to spawn")
	runCmd.Flags().String("namespace", "default", "Kubernetes namespace (for k8s mode)")
	runCmd.Flags().Duration("interval", 1*time.Minute, "Polling interval")
	runCmd.Flags().String("agent-provider", "openrouter", "Provider for spawned agents")
	runCmd.Flags().String("agent-model", "meta-llama/llama-3.3-70b-instruct:free", "Model for spawned agents")
	runCmd.Flags().String("image-pull-policy", "Always", "Image pull policy for agents (Always, IfNotPresent, Never)")

	runCmd.Flags().String("jira-query", "", "Custom JQL query (overrides label)")
	runCmd.Flags().String("poller", "jira", "Poller type: 'jira', 'github', 'file', or 'file-dir'")
	runCmd.Flags().String("watch-dir", "", "Directory to watch for work item files (for 'file-dir' poller)")

	runCmd.Flags().String("github-token", "", "GitHub API Token (for 'github' poller)")
	runCmd.Flags().String("github-owner", "", "GitHub Repository Owner (for 'github' poller)")
	runCmd.Flags().String("github-repo", "", "GitHub Repository Name (for 'github' poller)")
	runCmd.Flags().String("github-label", "", "GitHub Label to poll for (defaults to jira-label if not set)")

	// Bind flags
	viper.BindPFlag("orchestrator.jira_query", runCmd.Flags().Lookup("jira-query"))
	viper.BindPFlag("orchestrator.poller", runCmd.Flags().Lookup("poller"))
	viper.BindPFlag("orchestrator.watch_dir", runCmd.Flags().Lookup("watch-dir"))

	viper.BindPFlag("orchestrator.github_token", runCmd.Flags().Lookup("github-token"))
	viper.BindPFlag("orchestrator.github_owner", runCmd.Flags().Lookup("github-owner"))
	viper.BindPFlag("orchestrator.github_repo", runCmd.Flags().Lookup("github-repo"))
	viper.BindPFlag("orchestrator.github_label", runCmd.Flags().Lookup("github-label"))

	viper.BindPFlag("orchestrator.mode", runCmd.Flags().Lookup("mode"))
	viper.BindPFlag("orchestrator.jira_label", runCmd.Flags().Lookup("jira-label"))
	viper.BindPFlag("orchestrator.image", runCmd.Flags().Lookup("image"))
	viper.BindPFlag("orchestrator.namespace", runCmd.Flags().Lookup("namespace"))
	viper.BindPFlag("orchestrator.interval", runCmd.Flags().Lookup("interval"))
	viper.BindPFlag("orchestrator.agent_provider", runCmd.Flags().Lookup("agent-provider"))
	viper.BindPFlag("orchestrator.agent_model", runCmd.Flags().Lookup("agent-model"))
	viper.BindPFlag("orchestrator.image_pull_policy", runCmd.Flags().Lookup("image-pull-policy"))

	// Explicitly bind cleaner env vars
	viper.BindEnv("orchestrator.agent_provider", "RECAC_AGENT_PROVIDER")
	viper.BindEnv("orchestrator.agent_model", "RECAC_AGENT_MODEL")
	viper.BindEnv("orchestrator.poller", "RECAC_POLLER")
	viper.BindEnv("orchestrator.watch_dir", "RECAC_WATCH_DIR")
	viper.BindEnv("orchestrator.github_token", "RECAC_GITHUB_TOKEN", "GITHUB_TOKEN")
	viper.BindEnv("orchestrator.github_owner", "RECAC_GITHUB_OWNER")
	viper.BindEnv("orchestrator.github_repo", "RECAC_GITHUB_REPO")
	viper.BindEnv("orchestrator.github_label", "RECAC_GITHUB_LABEL")
	viper.BindEnv("orchestrator.mode", "RECAC_ORCHESTRATOR_MODE")
	viper.BindEnv("orchestrator.image", "RECAC_ORCHESTRATOR_IMAGE")
	viper.BindEnv("orchestrator.namespace", "RECAC_ORCHESTRATOR_NAMESPACE")
	viper.BindEnv("orchestrator.interval", "RECAC_ORCHESTRATOR_INTERVAL")
	viper.BindEnv("orchestrator.image_pull_policy", "RECAC_IMAGE_PULL_POLICY")
	viper.BindEnv("orchestrator.max_iterations", "RECAC_MAX_ITERATIONS")
	viper.BindEnv("orchestrator.manager_frequency", "RECAC_MANAGER_FREQUENCY")
	viper.BindEnv("orchestrator.task_max_iterations", "RECAC_TASK_MAX_ITERATIONS")
}

func runOrchestrator(cmd *cobra.Command, args []string) {
	logger := telemetry.NewLogger(viper.GetBool("verbose"), "orchestrator", false)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Setup Logic
	mode := viper.GetString("orchestrator.mode")
	image := viper.GetString("orchestrator.image")
	label := viper.GetString("orchestrator.jira_label")
	namespace := viper.GetString("orchestrator.namespace")
	interval := viper.GetDuration("orchestrator.interval")
	agentProvider := viper.GetString("orchestrator.agent_provider")

	query := viper.GetString("orchestrator.jira_query")
	logger.Info("Starting Orchestrator", "mode", mode, "label", label, "query", query, "interval", interval, "agent_provider", agentProvider)

	// 1. Poller
	var poller orchestrator.Poller
	pollerType := viper.GetString("orchestrator.poller")

	switch pollerType {
	case "file-dir":
		watchDir := viper.GetString("orchestrator.watch_dir")
		if watchDir == "" {
			logger.Error("Watch directory must be specified in file-dir poller mode")
			os.Exit(1)
		}
		var err error
		poller, err = orchestrator.NewFileDirPoller(watchDir)
		if err != nil {
			logger.Error("Failed to initialize file directory poller", "error", err)
			os.Exit(1)
		}
		logger.Info("Using file directory poller", "directory", watchDir)
	case "file", "filesystem":
		workFile := viper.GetString("orchestrator.work_file")
		if workFile == "" {
			logger.Error("Work file must be specified in file poller mode")
			os.Exit(1)
		}
		poller = orchestrator.NewFilePoller(workFile)
		logger.Info("Using filesystem poller", "file", workFile)
	case "github":
		token := viper.GetString("orchestrator.github_token")
		owner := viper.GetString("orchestrator.github_owner")
		repo := viper.GetString("orchestrator.github_repo")
		ghLabel := viper.GetString("orchestrator.github_label")
		if ghLabel == "" {
			ghLabel = label // Fallback to jira-label
		}

		if token == "" || owner == "" || repo == "" {
			logger.Error("GitHub token, owner, and repo must be specified in github poller mode")
			os.Exit(1)
		}
		poller = orchestrator.NewGitHubPoller(token, owner, repo, ghLabel)
		logger.Info("Using GitHub poller", "owner", owner, "repo", repo, "label", ghLabel)
	default:
		// Default to Jira
		jClient, err := cmdutils.GetJiraClient(ctx) // Use shared cmdutils
		if err != nil {
			logger.Error("Failed to initialize Jira client", "error", err)
			os.Exit(1)
		}
		jql := viper.GetString("orchestrator.jira_query")
		if jql == "" && label != "" {
			jql = fmt.Sprintf("labels = \"%s\" AND statusCategory != Done ORDER BY created ASC", label)
		}
		poller = orchestrator.NewJiraPoller(jClient, jql)
		logger.Info("Using Jira poller", "label", label, "query", jql)
	}

	// 2. Spawner
	var spawner orchestrator.Spawner
	var err error
	agentModel := viper.GetString("orchestrator.agent_model")

	switch mode {
	case "k8s", "kubernetes":
		pullPolicy := corev1.PullPolicy(viper.GetString("orchestrator.image_pull_policy"))
		if pullPolicy == "" {
			pullPolicy = corev1.PullAlways
		}
		spawner, err = orchestrator.NewK8sSpawner(logger, image, namespace, agentProvider, agentModel, pullPolicy)
		if err != nil {
			logger.Error("Failed to initialize K8s spawner", "error", err)
			os.Exit(1)
		}
	case "local", "docker":
		projectName := "recac-orchestrator" // Or similar
		dockerCli, err := docker.NewClient(projectName)
		if err != nil {
			logger.Error("Failed to initialize Docker client", "error", err)
			os.Exit(1)
		}

		sm, err := runner.NewSessionManager()
		if err != nil {
			logger.Error("Failed to initialize Session Manager", "error", err)
			os.Exit(1)
		}

		spawner = orchestrator.NewDockerSpawner(logger, dockerCli, image, projectName, poller, agentProvider, agentModel, sm)
	default:
		logger.Error("Invalid mode. Use 'local' or 'k8s'", "mode", mode)
		os.Exit(1)
	}

	// 3. Orchestrator
	orch := orchestrator.New(poller, spawner, interval)
	if err := orch.Run(ctx, logger); err != nil {
		if ctx.Err() != nil {
			// Graceful shutdown
			return
		}
		logger.Error("Orchestrator failure", "error", err)
		os.Exit(1)
	}
}
