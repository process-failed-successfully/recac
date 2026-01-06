package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"recac/internal/docker"
	"recac/internal/orchestrator"
	"recac/internal/telemetry"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var orchestrateCmd = &cobra.Command{
	Use:   "orchestrate",
	Short: "Run the RECAC Orchestrator",
	Long:  "Run the RECAC Orchestrator to pool Jira tickets and spawn agents (locally or in Kubernetes).",
	Run: func(cmd *cobra.Command, args []string) {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()

		logger := telemetry.NewLogger(viper.GetBool("debug"), "orchestrator")

		// Config
		mode := viper.GetString("orchestrator.mode")
		image := viper.GetString("orchestrator.image")
		label := viper.GetString("orchestrator.jira_label")
		namespace := viper.GetString("orchestrator.namespace")
		interval := viper.GetDuration("orchestrator.interval") // e.g. "1m"
		agentProvider := viper.GetString("orchestrator.agent_provider")
		agentModel := viper.GetString("orchestrator.agent_model")

		query := viper.GetString("orchestrator.jira_query")
		logger.Info("Starting Orchestrator", "mode", mode, "label", label, "query", query, "interval", interval, "agent_provider", agentProvider)

		// 1. Jira Client
		jClient, err := getJiraClient(ctx)
		if err != nil {
			logger.Error("Failed to initialize Jira client", "error", err)
			os.Exit(1)
		}

		// 2. Poller
		jql := viper.GetString("orchestrator.jira_query")
		if jql == "" && label != "" {
			jql = fmt.Sprintf("labels = \"%s\" AND statusCategory != Done ORDER BY created ASC", label)
		}
		poller := orchestrator.NewJiraPoller(jClient, jql)

		// 3. Spawner
		var spawner orchestrator.Spawner
		switch mode {
		case "k8s", "kubernetes":
			spawner, err = orchestrator.NewK8sSpawner(logger, image, namespace, agentProvider, agentModel)
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
			spawner = orchestrator.NewDockerSpawner(logger, dockerCli, image, projectName, poller, agentProvider, agentModel)
		default:
			logger.Error("Invalid mode. Use 'local' or 'k8s'", "mode", mode)
			os.Exit(1)
		}

		// 4. Orchestrator
		orch := orchestrator.New(poller, spawner, interval)
		if err := orch.Run(ctx, logger); err != nil {
			if ctx.Err() != nil {
				// Graceful shutdown
				return
			}
			logger.Error("Orchestrator failure", "error", err)
			os.Exit(1)
		}
	},
}

func init() {
	orchestrateCmd.Flags().String("mode", "local", "Orchestrator mode: 'local' (Docker) or 'k8s' (Kubernetes Job)")
	orchestrateCmd.Flags().String("jira-label", "recac-agent", "Jira label to poll for")
	orchestrateCmd.Flags().String("image", "ghcr.io/process-failed-successfully/recac-agent:latest", "Agent image to spawn")
	orchestrateCmd.Flags().String("namespace", "default", "Kubernetes namespace (for k8s mode)")
	orchestrateCmd.Flags().Duration("interval", 1*time.Minute, "Polling interval")
	orchestrateCmd.Flags().String("agent-provider", "openrouter", "Provider for spawned agents")
	orchestrateCmd.Flags().String("agent-model", "mistralai/devstral-2512:free", "Model for spawned agents")

	orchestrateCmd.Flags().String("jira-query", "", "Custom JQL query (overrides label)")
	viper.BindPFlag("orchestrator.jira_query", orchestrateCmd.Flags().Lookup("jira-query"))

	viper.BindPFlag("orchestrator.mode", orchestrateCmd.Flags().Lookup("mode"))
	viper.BindPFlag("orchestrator.jira_label", orchestrateCmd.Flags().Lookup("jira-label"))
	viper.BindPFlag("orchestrator.image", orchestrateCmd.Flags().Lookup("image"))
	viper.BindPFlag("orchestrator.namespace", orchestrateCmd.Flags().Lookup("namespace"))
	viper.BindPFlag("orchestrator.interval", orchestrateCmd.Flags().Lookup("interval"))
	viper.BindPFlag("orchestrator.agent_provider", orchestrateCmd.Flags().Lookup("agent-provider"))
	viper.BindPFlag("orchestrator.agent_model", orchestrateCmd.Flags().Lookup("agent-model"))

	// Explicitly bind cleaner env vars
	viper.BindEnv("orchestrator.agent_provider", "RECAC_AGENT_PROVIDER")
	viper.BindEnv("orchestrator.agent_model", "RECAC_AGENT_MODEL")

	rootCmd.AddCommand(orchestrateCmd)
}
