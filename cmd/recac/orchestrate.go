package main

import (
	"context"
	"fmt"
	"net/http"
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

var orchestrateCmd = &cobra.Command{
	Use:   "orchestrate",
	Short: "Run the RECAC Orchestrator",
	Long:  "Run the RECAC Orchestrator to pool Jira tickets and spawn agents (locally or in Kubernetes).",
	Run: func(cmd *cobra.Command, args []string) {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()

		var err error

		logger := telemetry.NewLogger(viper.GetBool("verbose"), "orchestrator", false)

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
		default:
			// Default to Jira
			jClient, err := cmdutils.GetJiraClient(ctx)
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

		// 3. Spawner
		maxIterations := viper.GetInt("orchestrator.max_iterations")
		managerFrequency := viper.GetInt("orchestrator.manager_frequency")
		taskMaxIterations := viper.GetInt("orchestrator.task_max_iterations")

		// Initialize SessionManager early to be available for both modes
		sm, err := runner.NewSessionManager()
		if err != nil {
			logger.Error("Failed to initialize Session Manager", "error", err)
			os.Exit(1)
		}

		var spawner orchestrator.Spawner
		switch mode {
		case "k8s", "kubernetes":
			pullPolicy := corev1.PullPolicy(viper.GetString("orchestrator.image_pull_policy"))
			if pullPolicy == "" {
				pullPolicy = corev1.PullAlways
			}
			spawner, err = orchestrator.NewK8sSpawner(logger, image, namespace, agentProvider, agentModel, pullPolicy, sm, maxIterations, managerFrequency, taskMaxIterations)
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

			pullPolicy := viper.GetString("orchestrator.image_pull_policy")
			spawner = orchestrator.NewDockerSpawner(logger, dockerCli, image, projectName, poller, agentProvider, agentModel, pullPolicy, sm, maxIterations, managerFrequency, taskMaxIterations)
		default:
			logger.Error("Invalid mode. Use 'local' or 'k8s'", "mode", mode)
			os.Exit(1)
		}

		// 4. Orchestrator
		orch := orchestrator.New(poller, spawner, interval)

		// Start Metrics Server
		metricsPort := viper.GetInt("orchestrator.metrics_port")
		statusHandler := func(mux *http.ServeMux) {
			orchestrator.RegisterAPI(mux, orch, logger, ctx)
		}

		metricsServer, actualPort, err := telemetry.StartMetricsServer(metricsPort, statusHandler)
		if err != nil {
			logger.Error("Failed to start metrics server", "error", err)
		} else {
			logger.Info("Metrics server started", "port", actualPort)
			defer func() {
				shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				if err := metricsServer.Shutdown(shutdownCtx); err != nil {
					logger.Error("Metrics server shutdown error", "error", err)
				}
			}()
		}

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
	orchestrateCmd.Flags().Int("metrics-port", 2112, "Port to expose Prometheus metrics and API")
	orchestrateCmd.Flags().String("mode", "local", "Orchestrator mode: 'local' (Docker) or 'k8s' (Kubernetes Job)")
	orchestrateCmd.Flags().String("jira-label", "recac-agent", "Jira label to poll for")
	orchestrateCmd.Flags().String("image", "ghcr.io/process-failed-successfully/recac-agent:latest", "Agent image to spawn")
	orchestrateCmd.Flags().String("namespace", "default", "Kubernetes namespace (for k8s mode)")
	orchestrateCmd.Flags().Duration("interval", 1*time.Minute, "Polling interval")
	orchestrateCmd.Flags().String("agent-provider", orchestrator.DefaultAgentProvider, "Provider for spawned agents")
	orchestrateCmd.Flags().String("agent-model", orchestrator.DefaultAgentModel, "Model for spawned agents")
	orchestrateCmd.Flags().String("image-pull-policy", "Always", "Image pull policy for agents (Always, IfNotPresent, Never)")

	orchestrateCmd.Flags().Int("max-iterations", 30, "Maximum number of iterations")
	orchestrateCmd.Flags().Int("manager-frequency", 5, "Frequency of manager reviews")
	orchestrateCmd.Flags().Int("task-max-iterations", 10, "Maximum iterations for sub-tasks")

	orchestrateCmd.Flags().String("jira-query", "", "Custom JQL query (overrides label)")
	orchestrateCmd.Flags().String("poller", "jira", "Poller type: 'jira', 'file', or 'file-dir'")
	orchestrateCmd.Flags().String("work-file", "work_items.json", "Work items file (for 'file' poller)")
	orchestrateCmd.Flags().String("watch-dir", "", "Directory to watch for work item files (for 'file-dir' poller)")

	viper.BindPFlag("orchestrator.jira_query", orchestrateCmd.Flags().Lookup("jira-query"))
	viper.BindPFlag("orchestrator.poller", orchestrateCmd.Flags().Lookup("poller"))
	viper.BindPFlag("orchestrator.work_file", orchestrateCmd.Flags().Lookup("work-file"))
	viper.BindPFlag("orchestrator.watch_dir", orchestrateCmd.Flags().Lookup("watch-dir"))

	viper.BindPFlag("orchestrator.mode", orchestrateCmd.Flags().Lookup("mode"))
	viper.BindPFlag("orchestrator.jira_label", orchestrateCmd.Flags().Lookup("jira-label"))
	viper.BindPFlag("orchestrator.image", orchestrateCmd.Flags().Lookup("image"))
	viper.BindPFlag("orchestrator.namespace", orchestrateCmd.Flags().Lookup("namespace"))
	viper.BindPFlag("orchestrator.interval", orchestrateCmd.Flags().Lookup("interval"))
	viper.BindPFlag("orchestrator.agent_provider", orchestrateCmd.Flags().Lookup("agent-provider"))
	viper.BindPFlag("orchestrator.agent_model", orchestrateCmd.Flags().Lookup("agent-model"))
	viper.BindPFlag("orchestrator.image_pull_policy", orchestrateCmd.Flags().Lookup("image-pull-policy"))

	viper.BindPFlag("orchestrator.max_iterations", orchestrateCmd.Flags().Lookup("max-iterations"))
	viper.BindPFlag("orchestrator.manager_frequency", orchestrateCmd.Flags().Lookup("manager-frequency"))
	viper.BindPFlag("orchestrator.task_max_iterations", orchestrateCmd.Flags().Lookup("task-max-iterations"))
	viper.BindPFlag("orchestrator.metrics_port", orchestrateCmd.Flags().Lookup("metrics-port"))

	// Explicitly bind cleaner env vars
	viper.BindEnv("orchestrator.agent_provider", "RECAC_AGENT_PROVIDER")
	viper.BindEnv("orchestrator.agent_model", "RECAC_AGENT_MODEL")
	viper.BindEnv("orchestrator.poller", "RECAC_POLLER")
	viper.BindEnv("orchestrator.work_file", "RECAC_WORK_FILE")
	viper.BindEnv("orchestrator.watch_dir", "RECAC_WATCH_DIR")
	viper.BindEnv("orchestrator.mode", "RECAC_ORCHESTRATOR_MODE")
	viper.BindEnv("orchestrator.image", "RECAC_ORCHESTRATOR_IMAGE")
	viper.BindEnv("orchestrator.namespace", "RECAC_ORCHESTRATOR_NAMESPACE")
	viper.BindEnv("orchestrator.interval", "RECAC_ORCHESTRATOR_INTERVAL")
	viper.BindEnv("orchestrator.image_pull_policy", "RECAC_IMAGE_PULL_POLICY")
	viper.BindEnv("orchestrator.max_iterations", "RECAC_MAX_ITERATIONS")
	viper.BindEnv("orchestrator.manager_frequency", "RECAC_MANAGER_FREQUENCY")
	viper.BindEnv("orchestrator.task_max_iterations", "RECAC_TASK_MAX_ITERATIONS")
	viper.BindEnv("orchestrator.metrics_port", "RECAC_METRICS_PORT")

	rootCmd.AddCommand(orchestrateCmd)
}
