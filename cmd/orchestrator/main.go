package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"recac/internal/cmdutils"
	"recac/internal/config"
	"recac/internal/docker"
	"recac/internal/orchestrator"
	"recac/internal/runner"
	"recac/internal/telemetry"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	corev1 "k8s.io/api/core/v1"
)

func main() {
	// Flags
	var cfgFile string
	pflag.StringVar(&cfgFile, "config", "", "config file (default is $HOME/.recac.yaml)")
	pflag.BoolP("verbose", "v", false, "Enable verbose/debug logging")

	pflag.String("mode", "local", "Orchestrator mode: 'local' (Docker) or 'k8s' (Kubernetes Job)")
	pflag.String("jira-label", "recac-agent", "Jira label to poll for")
	pflag.String("image", "ghcr.io/process-failed-successfully/recac-agent:latest", "Agent image to spawn")
	pflag.String("namespace", "default", "Kubernetes namespace (for k8s mode)")
	pflag.Duration("interval", 1*time.Minute, "Polling interval")
	pflag.String("agent-provider", "openrouter", "Provider for spawned agents")
	pflag.String("agent-model", "mistralai/devstral-2512", "Model for spawned agents")
	pflag.String("image-pull-policy", "Always", "Image pull policy for agents (Always, IfNotPresent, Never)")

	pflag.String("jira-query", "", "Custom JQL query (overrides label)")
	pflag.String("poller", "jira", "Poller type: 'jira', 'github', 'file', or 'file-dir'")
	pflag.String("work-file", "work_items.json", "Work items file (for 'file' poller)")
	pflag.String("watch-dir", "", "Directory to watch for work item files (for 'file-dir' poller)")

	pflag.String("github-token", "", "GitHub API Token (for 'github' poller)")
	pflag.String("github-owner", "", "GitHub Repository Owner (for 'github' poller)")
	pflag.String("github-repo", "", "GitHub Repository Name (for 'github' poller)")
	pflag.String("github-label", "", "GitHub Label to poll for (defaults to jira-label if not set)")

	pflag.Parse()

	// Config
	config.Load(cfgFile)

	// Bind Flags
	viper.BindPFlag("verbose", pflag.Lookup("verbose"))
	viper.BindPFlag("orchestrator.jira_query", pflag.Lookup("jira-query"))
	viper.BindPFlag("orchestrator.poller", pflag.Lookup("poller"))
	viper.BindPFlag("orchestrator.work_file", pflag.Lookup("work-file"))
	viper.BindPFlag("orchestrator.watch_dir", pflag.Lookup("watch-dir"))

	viper.BindPFlag("orchestrator.github_token", pflag.Lookup("github-token"))
	viper.BindPFlag("orchestrator.github_owner", pflag.Lookup("github-owner"))
	viper.BindPFlag("orchestrator.github_repo", pflag.Lookup("github-repo"))
	viper.BindPFlag("orchestrator.github_label", pflag.Lookup("github-label"))

	viper.BindPFlag("orchestrator.mode", pflag.Lookup("mode"))
	viper.BindPFlag("orchestrator.jira_label", pflag.Lookup("jira-label"))
	viper.BindPFlag("orchestrator.image", pflag.Lookup("image"))
	viper.BindPFlag("orchestrator.namespace", pflag.Lookup("namespace"))
	viper.BindPFlag("orchestrator.interval", pflag.Lookup("interval"))
	viper.BindPFlag("orchestrator.agent_provider", pflag.Lookup("agent-provider"))
	viper.BindPFlag("orchestrator.agent_model", pflag.Lookup("agent-model"))
	viper.BindPFlag("orchestrator.image_pull_policy", pflag.Lookup("image-pull-policy"))

	// Explicitly bind cleaner env vars
	viper.BindEnv("orchestrator.agent_provider", "RECAC_AGENT_PROVIDER")
	viper.BindEnv("orchestrator.agent_model", "RECAC_AGENT_MODEL")
	viper.BindEnv("orchestrator.poller", "RECAC_POLLER")
	viper.BindEnv("orchestrator.work_file", "RECAC_WORK_FILE")
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

	// Logger
	logger := telemetry.NewLogger(viper.GetBool("verbose"), "orchestrator", false)
	telemetry.InitLogger(viper.GetBool("verbose"), "orchestrator", false) // Ensure global logger is set

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Setup Logic
	mode := viper.GetString("orchestrator.mode")
	image := viper.GetString("orchestrator.image")
	label := viper.GetString("orchestrator.jira_label")
	namespace := viper.GetString("orchestrator.namespace")
	interval := viper.GetDuration("orchestrator.interval") // e.g. "1m"
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
