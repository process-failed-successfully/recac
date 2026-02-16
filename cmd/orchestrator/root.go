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

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	corev1 "k8s.io/api/core/v1"
)

var (
	cfgFile string
	verbose bool
)

var rootCmd = &cobra.Command{
	Use:   "orchestrator",
	Short: "RECAC Orchestrator",
	Long:  `The Orchestrator polls for work (Jira, GitHub, Files) and spawns Agents (Docker, K8s).`,
	RunE:  runOrchestrator,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	// Persistent Flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.recac.yaml)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose/debug logging")
	rootCmd.PersistentFlags().String("mode", "local", "Orchestrator mode: 'local' (Docker) or 'k8s' (Kubernetes Job)")

	// Bind Persistent Flags
	viper.BindPFlag("verbose", rootCmd.PersistentFlags().Lookup("verbose"))
	viper.BindPFlag("orchestrator.mode", rootCmd.PersistentFlags().Lookup("mode"))

	// Local Flags (for run command, which is root)
	rootCmd.Flags().String("jira-label", "recac-agent", "Jira label to poll for")
	rootCmd.Flags().String("image", "ghcr.io/process-failed-successfully/recac-agent:latest", "Agent image to spawn")
	rootCmd.Flags().String("namespace", "default", "Kubernetes namespace (for k8s mode)")
	rootCmd.Flags().Duration("interval", 1*time.Minute, "Polling interval")
	rootCmd.Flags().String("agent-provider", "openrouter", "Provider for spawned agents")
	rootCmd.Flags().String("agent-model", "meta-llama/llama-3.3-70b-instruct:free", "Model for spawned agents")
	rootCmd.Flags().String("image-pull-policy", "Always", "Image pull policy for agents (Always, IfNotPresent, Never)")

	rootCmd.Flags().String("jira-query", "", "Custom JQL query (overrides label)")
	rootCmd.Flags().String("poller", "jira", "Poller type: 'jira', 'github', 'file', or 'file-dir'")
	rootCmd.Flags().String("work-file", "work_items.json", "Work items file (for 'file' poller)")
	rootCmd.Flags().String("watch-dir", "", "Directory to watch for work item files (for 'file-dir' poller)")

	rootCmd.Flags().String("github-token", "", "GitHub API Token (for 'github' poller)")
	rootCmd.Flags().String("github-owner", "", "GitHub Repository Owner (for 'github' poller)")
	rootCmd.Flags().String("github-repo", "", "GitHub Repository Name (for 'github' poller)")
	rootCmd.Flags().String("github-label", "", "GitHub Label to poll for (defaults to jira-label if not set)")

	// Bind Flags
	viper.BindPFlag("orchestrator.jira_query", rootCmd.Flags().Lookup("jira-query"))
	viper.BindPFlag("orchestrator.poller", rootCmd.Flags().Lookup("poller"))
	viper.BindPFlag("orchestrator.work_file", rootCmd.Flags().Lookup("work-file"))
	viper.BindPFlag("orchestrator.watch_dir", rootCmd.Flags().Lookup("watch-dir"))

	viper.BindPFlag("orchestrator.github_token", rootCmd.Flags().Lookup("github-token"))
	viper.BindPFlag("orchestrator.github_owner", rootCmd.Flags().Lookup("github-owner"))
	viper.BindPFlag("orchestrator.github_repo", rootCmd.Flags().Lookup("github-repo"))
	viper.BindPFlag("orchestrator.github_label", rootCmd.Flags().Lookup("github-label"))

	viper.BindPFlag("orchestrator.jira_label", rootCmd.Flags().Lookup("jira-label"))
	viper.BindPFlag("orchestrator.image", rootCmd.Flags().Lookup("image"))
	viper.BindPFlag("orchestrator.namespace", rootCmd.Flags().Lookup("namespace"))
	viper.BindPFlag("orchestrator.interval", rootCmd.Flags().Lookup("interval"))
	viper.BindPFlag("orchestrator.agent_provider", rootCmd.Flags().Lookup("agent-provider"))
	viper.BindPFlag("orchestrator.agent_model", rootCmd.Flags().Lookup("agent-model"))
	viper.BindPFlag("orchestrator.image_pull_policy", rootCmd.Flags().Lookup("image-pull-policy"))

	// Explicitly bind cleaner env vars (copied from main.go)
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
}

func initConfig() {
	config.Load(cfgFile)
	telemetry.InitLogger(viper.GetBool("verbose"), "orchestrator", false)
}

func runOrchestrator(cmd *cobra.Command, args []string) error {
	// Logger
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
			return fmt.Errorf("watch directory must be specified in file-dir poller mode")
		}
		var err error
		poller, err = orchestrator.NewFileDirPoller(watchDir)
		if err != nil {
			return fmt.Errorf("failed to initialize file directory poller: %w", err)
		}
		logger.Info("Using file directory poller", "directory", watchDir)
	case "file", "filesystem":
		workFile := viper.GetString("orchestrator.work_file")
		if workFile == "" {
			return fmt.Errorf("work file must be specified in file poller mode")
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
			return fmt.Errorf("GitHub token, owner, and repo must be specified in github poller mode")
		}
		poller = orchestrator.NewGitHubPoller(token, owner, repo, ghLabel)
		logger.Info("Using GitHub poller", "owner", owner, "repo", repo, "label", ghLabel)
	default:
		// Default to Jira
		jClient, err := cmdutils.GetJiraClient(ctx) // Use shared cmdutils
		if err != nil {
			return fmt.Errorf("failed to initialize Jira client: %w", err)
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
			return fmt.Errorf("failed to initialize K8s spawner: %w", err)
		}
	case "local", "docker":
		projectName := "recac-orchestrator" // Or similar
		dockerCli, err := docker.NewClient(projectName)
		if err != nil {
			return fmt.Errorf("failed to initialize Docker client: %w", err)
		}

		sm, err := runner.NewSessionManager()
		if err != nil {
			return fmt.Errorf("failed to initialize Session Manager: %w", err)
		}

		spawner = orchestrator.NewDockerSpawner(logger, dockerCli, image, projectName, poller, agentProvider, agentModel, sm)
	default:
		return fmt.Errorf("invalid mode. Use 'local' or 'k8s', got: %s", mode)
	}

	// 3. Orchestrator
	orch := orchestrator.New(poller, spawner, interval)
	if err := orch.Run(ctx, logger); err != nil {
		if ctx.Err() != nil {
			// Graceful shutdown
			return nil
		}
		return fmt.Errorf("orchestrator failure: %w", err)
	}
	return nil
}
