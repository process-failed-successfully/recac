package main

import (
	"fmt"
	"os"
	"time"

	"recac/internal/config"
	"recac/internal/telemetry"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string

var rootCmd = &cobra.Command{
	Use:   "orchestrator",
	Short: "RECAC Orchestrator",
	Long:  "The Orchestrator manages the task pool and spawns agents to handle tasks.",
	Run: func(cmd *cobra.Command, args []string) {
		// Default behavior: Run the orchestrator loop
		runOrchestrator(cmd, args)
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	// Global Flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.recac.yaml)")
	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "Enable verbose/debug logging")

	// Orchestrator Flags (Persistent because dashboard might need some of them, e.g. mode/namespace)
	rootCmd.PersistentFlags().String("mode", "local", "Orchestrator mode: 'local' (Docker) or 'k8s' (Kubernetes Job)")
	rootCmd.PersistentFlags().String("jira-label", "recac-agent", "Jira label to poll for")
	rootCmd.PersistentFlags().String("image", "ghcr.io/process-failed-successfully/recac-agent:latest", "Agent image to spawn")
	rootCmd.PersistentFlags().String("namespace", "default", "Kubernetes namespace (for k8s mode)")
	rootCmd.PersistentFlags().Duration("interval", 1*time.Minute, "Polling interval")
	rootCmd.PersistentFlags().String("agent-provider", "openrouter", "Provider for spawned agents")
	rootCmd.PersistentFlags().String("agent-model", "meta-llama/llama-3.3-70b-instruct:free", "Model for spawned agents")
	rootCmd.PersistentFlags().String("image-pull-policy", "Always", "Image pull policy for agents (Always, IfNotPresent, Never)")

	rootCmd.PersistentFlags().String("jira-query", "", "Custom JQL query (overrides label)")
	rootCmd.PersistentFlags().String("poller", "jira", "Poller type: 'jira', 'github', 'file', or 'file-dir'")
	rootCmd.PersistentFlags().String("work-file", "work_items.json", "Work items file (for 'file' poller)")
	rootCmd.PersistentFlags().String("watch-dir", "", "Directory to watch for work item files (for 'file-dir' poller)")

	rootCmd.PersistentFlags().String("github-token", "", "GitHub API Token (for 'github' poller)")
	rootCmd.PersistentFlags().String("github-owner", "", "GitHub Repository Owner (for 'github' poller)")
	rootCmd.PersistentFlags().String("github-repo", "", "GitHub Repository Name (for 'github' poller)")
	rootCmd.PersistentFlags().String("github-label", "", "GitHub Label to poll for (defaults to jira-label if not set)")

	// Bind Flags
	viper.BindPFlag("verbose", rootCmd.PersistentFlags().Lookup("verbose"))
	viper.BindPFlag("orchestrator.jira_query", rootCmd.PersistentFlags().Lookup("jira-query"))
	viper.BindPFlag("orchestrator.poller", rootCmd.PersistentFlags().Lookup("poller"))
	viper.BindPFlag("orchestrator.work_file", rootCmd.PersistentFlags().Lookup("work-file"))
	viper.BindPFlag("orchestrator.watch_dir", rootCmd.PersistentFlags().Lookup("watch-dir"))

	viper.BindPFlag("orchestrator.github_token", rootCmd.PersistentFlags().Lookup("github-token"))
	viper.BindPFlag("orchestrator.github_owner", rootCmd.PersistentFlags().Lookup("github-owner"))
	viper.BindPFlag("orchestrator.github_repo", rootCmd.PersistentFlags().Lookup("github-repo"))
	viper.BindPFlag("orchestrator.github_label", rootCmd.PersistentFlags().Lookup("github-label"))

	viper.BindPFlag("orchestrator.mode", rootCmd.PersistentFlags().Lookup("mode"))
	viper.BindPFlag("orchestrator.jira_label", rootCmd.PersistentFlags().Lookup("jira-label"))
	viper.BindPFlag("orchestrator.image", rootCmd.PersistentFlags().Lookup("image"))
	viper.BindPFlag("orchestrator.namespace", rootCmd.PersistentFlags().Lookup("namespace"))
	viper.BindPFlag("orchestrator.interval", rootCmd.PersistentFlags().Lookup("interval"))
	viper.BindPFlag("orchestrator.agent_provider", rootCmd.PersistentFlags().Lookup("agent-provider"))
	viper.BindPFlag("orchestrator.agent_model", rootCmd.PersistentFlags().Lookup("agent-model"))
	viper.BindPFlag("orchestrator.image_pull_policy", rootCmd.PersistentFlags().Lookup("image-pull-policy"))

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
}

func initConfig() {
	config.Load(cfgFile)

	// Logger
	telemetry.InitLogger(viper.GetBool("verbose"), "orchestrator", false)
}
