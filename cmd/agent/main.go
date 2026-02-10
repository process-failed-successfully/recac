package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"recac/internal/cmdutils"
	"recac/internal/config"
	"recac/internal/telemetry"
	"recac/internal/workflow"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

func initFlags(cfgFile *string) {
	pflag.StringVar(cfgFile, "config", "", "config file (default is $HOME/.recac.yaml)")
	pflag.BoolP("verbose", "v", false, "Enable verbose/debug logging")

	// Session Flags
	pflag.String("path", "", "Project path")
	pflag.Int("max-iterations", 30, "Maximum number of iterations")
	pflag.Int("manager-frequency", 5, "Frequency of manager reviews")
	pflag.Int("max-agents", 1, "Maximum number of parallel agents")
	pflag.Int("task-max-iterations", 10, "Maximum iterations for sub-tasks")
	pflag.Bool("detached", false, "Run session in background (detached mode)")
	pflag.String("name", "", "Name for the session (required for detached mode)")
	pflag.String("jira", "", "Jira Ticket ID to start session from (e.g. PROJ-123)")
	pflag.Bool("manager-first", false, "Run the Manager Agent before the first coding session")
	pflag.Bool("stream", false, "Stream agent output to the console")
	pflag.Bool("allow-dirty", false, "Allow running with uncommitted git changes")

	pflag.Bool("auto-merge", false, "Automatically merge PRs if checks pass")
	pflag.Bool("skip-qa", false, "Skip QA phase and auto-complete (use with caution)")
	pflag.String("image", "ghcr.io/process-failed-successfully/recac-agent:latest", "Docker image to use for the agent session")
	pflag.Bool("cleanup", true, "Cleanup temporary workspace after session ends")
	pflag.String("project", "", "Project name override")

	pflag.String("repo-url", "", "Repository URL to clone (bypasses Jira if provided)")
	pflag.String("summary", "", "Task summary (bypasses Jira if provided)")
	pflag.String("description", "", "Task description")

	pflag.String("provider", "", "Agent provider override")
	pflag.String("model", "", "Agent model override")
	pflag.Bool("mock", false, "Mock mode")
}

func runApp(ctx context.Context) error {

	// Bindings
	viper.BindPFlag("verbose", pflag.Lookup("verbose"))
	viper.BindPFlag("path", pflag.Lookup("path"))
	viper.BindPFlag("max_iterations", pflag.Lookup("max-iterations"))
	viper.BindPFlag("manager_frequency", pflag.Lookup("manager-frequency"))
	viper.BindPFlag("max_agents", pflag.Lookup("max-agents"))
	viper.BindPFlag("task_max_iterations", pflag.Lookup("task-max-iterations"))
	viper.BindPFlag("detached", pflag.Lookup("detached"))
	viper.BindPFlag("name", pflag.Lookup("name"))
	viper.BindPFlag("jira", pflag.Lookup("jira"))
	viper.BindPFlag("manager_first", pflag.Lookup("manager-first"))
	viper.BindPFlag("stream", pflag.Lookup("stream"))
	viper.BindPFlag("allow_dirty", pflag.Lookup("allow-dirty"))
	viper.BindPFlag("auto_merge", pflag.Lookup("auto-merge"))
	viper.BindPFlag("skip_qa", pflag.Lookup("skip-qa"))
	viper.BindPFlag("image", pflag.Lookup("image"))
	viper.BindPFlag("cleanup", pflag.Lookup("cleanup"))
	viper.BindPFlag("project", pflag.Lookup("project"))
	viper.BindPFlag("repo_url", pflag.Lookup("repo-url"))
	viper.BindPFlag("summary", pflag.Lookup("summary"))
	viper.BindPFlag("description", pflag.Lookup("description"))
	viper.BindPFlag("provider", pflag.Lookup("provider"))
	viper.BindPFlag("model", pflag.Lookup("model"))
	viper.BindPFlag("mock", pflag.Lookup("mock"))

	viper.BindEnv("max_iterations", "RECAC_MAX_ITERATIONS")
	viper.BindEnv("manager_frequency", "RECAC_MANAGER_FREQUENCY")
	viper.BindEnv("task_max_iterations", "RECAC_TASK_MAX_ITERATIONS")

	// Explicitly bind Provider/Model to ensure Env vars take precedence over config file
	viper.BindEnv("provider", "RECAC_PROVIDER", "RECAC_AGENT_PROVIDER")
	viper.BindEnv("model", "RECAC_MODEL", "RECAC_AGENT_MODEL")

	// Init Logger
	telemetry.InitLogger(viper.GetBool("verbose"), "", false)
	logger := telemetry.NewLogger(viper.GetBool("verbose"), "", false)

	// Debug config resolution
	logger.Info("Agent Configuration Resolved",
		"provider", viper.GetString("provider"),
		"model", viper.GetString("model"),
		"env_recac_provider", os.Getenv("RECAC_PROVIDER"),
	)

	// Construct SessionConfig
	cfg := workflow.SessionConfig{
		ProjectPath:       viper.GetString("path"),
		IsMock:            viper.GetBool("mock"),
		MaxIterations:     viper.GetInt("max_iterations"),
		ManagerFrequency:  viper.GetInt("manager_frequency"),
		MaxAgents:         viper.GetInt("max_agents"),
		TaskMaxIterations: viper.GetInt("task_max_iterations"),
		Detached:          viper.GetBool("detached"),
		SessionName:       viper.GetString("name"),
		AllowDirty:        viper.GetBool("allow_dirty"),
		Stream:            viper.GetBool("stream"),
		AutoMerge:         viper.GetBool("auto_merge"),
		SkipQA:            viper.GetBool("skip_qa"),
		ManagerFirst:      viper.GetBool("manager_first"),
		Image:             viper.GetString("image"),
		Debug:             viper.GetBool("verbose"),
		Provider:          viper.GetString("provider"),
		Model:             viper.GetString("model"),
		Cleanup:           viper.GetBool("cleanup"),
		ProjectName:       viper.GetString("project"),
		RepoURL:           viper.GetString("repo_url"),
		Summary:           viper.GetString("summary"),
		Description:       viper.GetString("description"),
		JiraTicketID:      viper.GetString("jira"),
		Logger:            logger,
		CommandPrefix:     []string{}, // Agent binary doesn't use subcommands, unless needed.
	}

	// Logic
	if cfg.JiraTicketID != "" {
		jClient, err := cmdutils.GetJiraClient(ctx)
		if err != nil {
			return err
		}
		if err := workflow.ProcessJiraTicket(ctx, cfg.JiraTicketID, jClient, cfg, nil); err != nil {
			return err
		}
		return nil
	}

	if cfg.RepoURL != "" {
		if err := workflow.ProcessDirectTask(ctx, cfg); err != nil {
			return err
		}
		return nil
	}

	// Normal Workflow
	if err := workflow.RunWorkflow(ctx, cfg); err != nil {
		return err
	}

	return nil
}

func main() {
	var cfgFile string
	initFlags(&cfgFile)
	pflag.Parse()
	config.Load(cfgFile)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := runApp(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
