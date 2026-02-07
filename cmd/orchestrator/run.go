package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

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
	Short: "Run the orchestrator loop",
	Run:   runOrchestrator,
}

func init() {
	rootCmd.AddCommand(runCmd)
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
