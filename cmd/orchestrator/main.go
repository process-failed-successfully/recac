package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"recac/internal/cmdutils"
	"recac/internal/config"
	"recac/internal/docker"
	"recac/internal/orchestrator"
	"recac/internal/runner"
	"recac/internal/telemetry"
	"recac/internal/tui"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	corev1 "k8s.io/api/core/v1"
)

func main() {
	// Flags
	var cfgFile string
	pflag.StringVar(&cfgFile, "config", "", "config file (default is $HOME/.recac.yaml)")
	pflag.BoolP("verbose", "v", false, "Enable verbose/debug logging")
	pflag.Bool("dry-run", false, "Poll for work items without spawning agents")
	pflag.Bool("verify", false, "Verify configuration and connectivity without running the loop")
	pflag.Bool("list-jobs", false, "List active jobs from a running orchestrator instance")
	pflag.Bool("history", false, "Include completed jobs in list-jobs")
	pflag.Bool("monitor", false, "Launch the TUI dashboard to monitor the orchestrator")
	pflag.String("logs", "", "Get logs for a specific job ID from a running orchestrator instance")
	pflag.String("inspect-job", "", "Inspect a specific job by ID")
	pflag.String("cancel-job", "", "Cancel a running job by ID")
	pflag.String("retry-job", "", "Retry a completed job by ID")
	pflag.Bool("retry-failed", false, "Retry all failed jobs from history")
	pflag.Bool("pause", false, "Pause the orchestrator polling loop")
	pflag.Bool("resume", false, "Resume the orchestrator polling loop")
	pflag.String("submit", "", "Submit a job from a JSON file path")
	pflag.String("submit-url", "", "Repo URL for ad-hoc job submission")
	pflag.String("submit-task", "", "Task description for ad-hoc job submission")
	pflag.String("submit-id", "", "Optional ID for ad-hoc job submission")
	pflag.Bool("wait", false, "Wait for job completion and stream logs (for submit/submit-url)")
	pflag.String("host", "http://localhost:2112", "Orchestrator host URL (for list-jobs, logs, cancel-job, and submit)")

	pflag.String("mode", "local", "Orchestrator mode: 'local' (Docker) or 'k8s' (Kubernetes Job)")
	pflag.String("jira-label", "recac-agent", "Jira label to poll for")
	pflag.String("image", "ghcr.io/process-failed-successfully/recac-agent:latest", "Agent image to spawn")
	pflag.String("namespace", "default", "Kubernetes namespace (for k8s mode)")
	pflag.Duration("interval", 1*time.Minute, "Polling interval")
	pflag.String("agent-provider", orchestrator.DefaultAgentProvider, "Provider for spawned agents")
	pflag.String("agent-model", orchestrator.DefaultAgentModel, "Model for spawned agents")
	pflag.String("image-pull-policy", "Always", "Image pull policy for agents (Always, IfNotPresent, Never)")
	pflag.Int("metrics-port", 2112, "Port to expose Prometheus metrics")
	pflag.String("db-file", "", "Path to SQLite database for job history persistence")

	pflag.Int("max-iterations", 30, "Maximum number of iterations")
	pflag.Int("manager-frequency", 5, "Frequency of manager reviews")
	pflag.Int("task-max-iterations", 10, "Maximum iterations for sub-tasks")

	// Janitor Flags
	pflag.Bool("cleanup", false, "Enable janitor to clean up old containers")
	pflag.Duration("cleanup-interval", 5*time.Minute, "Janitor check interval")
	pflag.Duration("cleanup-age", 24*time.Hour, "Age of containers to clean up")
	pflag.Bool("cleanup-dry-run", false, "Janitor dry run (log only)")

	pflag.String("jira-query", "", "Custom JQL query (overrides label)")
	pflag.String("poller", "jira", "Poller type: 'jira', 'github', 'gitlab', 'file', or 'file-dir'")
	pflag.String("work-file", "work_items.json", "Work items file (for 'file' poller)")
	pflag.String("watch-dir", "", "Directory to watch for work item files (for 'file-dir' poller)")

	pflag.String("github-token", "", "GitHub API Token (for 'github' poller)")
	pflag.String("github-owner", "", "GitHub Repository Owner (for 'github' poller)")
	pflag.String("github-repo", "", "GitHub Repository Name (for 'github' poller)")
	pflag.String("github-label", "", "GitHub Label to poll for (defaults to jira-label if not set)")

	pflag.String("gitlab-token", "", "GitLab API Token (for 'gitlab' poller)")
	pflag.String("gitlab-project", "", "GitLab Project ID or URL-encoded path (for 'gitlab' poller)")
	pflag.String("gitlab-label", "", "GitLab Label to poll for (defaults to jira-label if not set)")
	pflag.String("gitlab-url", "", "GitLab URL (defaults to https://gitlab.com)")

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

	viper.BindPFlag("orchestrator.gitlab_token", pflag.Lookup("gitlab-token"))
	viper.BindPFlag("orchestrator.gitlab_project", pflag.Lookup("gitlab-project"))
	viper.BindPFlag("orchestrator.gitlab_label", pflag.Lookup("gitlab-label"))
	viper.BindPFlag("orchestrator.gitlab_url", pflag.Lookup("gitlab-url"))

	viper.BindPFlag("orchestrator.dry_run", pflag.Lookup("dry-run"))
	viper.BindEnv("orchestrator.dry_run", "RECAC_ORCHESTRATOR_DRY_RUN")

	viper.BindPFlag("orchestrator.verify", pflag.Lookup("verify"))
	viper.BindPFlag("orchestrator.list_jobs", pflag.Lookup("list-jobs"))
	viper.BindPFlag("orchestrator.history", pflag.Lookup("history"))
	viper.BindPFlag("orchestrator.monitor", pflag.Lookup("monitor"))
	viper.BindPFlag("orchestrator.logs", pflag.Lookup("logs"))
	viper.BindPFlag("orchestrator.inspect_job", pflag.Lookup("inspect-job"))
	viper.BindPFlag("orchestrator.cancel_job", pflag.Lookup("cancel-job"))
	viper.BindPFlag("orchestrator.retry_job", pflag.Lookup("retry-job"))
	viper.BindPFlag("orchestrator.retry_failed", pflag.Lookup("retry-failed"))
	viper.BindPFlag("orchestrator.pause", pflag.Lookup("pause"))
	viper.BindPFlag("orchestrator.resume", pflag.Lookup("resume"))
	viper.BindPFlag("orchestrator.submit", pflag.Lookup("submit"))
	viper.BindPFlag("orchestrator.submit_url", pflag.Lookup("submit-url"))
	viper.BindPFlag("orchestrator.submit_task", pflag.Lookup("submit-task"))
	viper.BindPFlag("orchestrator.submit_id", pflag.Lookup("submit-id"))
	viper.BindPFlag("orchestrator.wait", pflag.Lookup("wait"))
	viper.BindPFlag("orchestrator.host", pflag.Lookup("host"))

	viper.BindPFlag("orchestrator.mode", pflag.Lookup("mode"))
	viper.BindPFlag("orchestrator.jira_label", pflag.Lookup("jira-label"))
	viper.BindPFlag("orchestrator.image", pflag.Lookup("image"))
	viper.BindPFlag("orchestrator.namespace", pflag.Lookup("namespace"))
	viper.BindPFlag("orchestrator.interval", pflag.Lookup("interval"))
	viper.BindPFlag("orchestrator.agent_provider", pflag.Lookup("agent-provider"))
	viper.BindPFlag("orchestrator.agent_model", pflag.Lookup("agent-model"))
	viper.BindPFlag("orchestrator.image_pull_policy", pflag.Lookup("image-pull-policy"))
	viper.BindPFlag("orchestrator.metrics_port", pflag.Lookup("metrics-port"))

	viper.BindPFlag("orchestrator.max_iterations", pflag.Lookup("max-iterations"))
	viper.BindPFlag("orchestrator.manager_frequency", pflag.Lookup("manager-frequency"))
	viper.BindPFlag("orchestrator.task_max_iterations", pflag.Lookup("task-max-iterations"))

	viper.BindPFlag("orchestrator.cleanup", pflag.Lookup("cleanup"))
	viper.BindPFlag("orchestrator.cleanup_interval", pflag.Lookup("cleanup-interval"))
	viper.BindPFlag("orchestrator.cleanup_age", pflag.Lookup("cleanup-age"))
	viper.BindPFlag("orchestrator.cleanup_dry_run", pflag.Lookup("cleanup-dry-run"))

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
	viper.BindEnv("orchestrator.gitlab_token", "RECAC_GITLAB_TOKEN", "GITLAB_TOKEN")
	viper.BindEnv("orchestrator.gitlab_project", "RECAC_GITLAB_PROJECT")
	viper.BindEnv("orchestrator.gitlab_label", "RECAC_GITLAB_LABEL")
	viper.BindEnv("orchestrator.gitlab_url", "RECAC_GITLAB_URL")
	viper.BindEnv("orchestrator.mode", "RECAC_ORCHESTRATOR_MODE")
	viper.BindEnv("orchestrator.image", "RECAC_ORCHESTRATOR_IMAGE")
	viper.BindEnv("orchestrator.namespace", "RECAC_ORCHESTRATOR_NAMESPACE")
	viper.BindEnv("orchestrator.interval", "RECAC_ORCHESTRATOR_INTERVAL")
	viper.BindEnv("orchestrator.image_pull_policy", "RECAC_IMAGE_PULL_POLICY")
	viper.BindEnv("orchestrator.metrics_port", "RECAC_METRICS_PORT")
	viper.BindPFlag("orchestrator.db_file", pflag.Lookup("db-file"))
	viper.BindEnv("orchestrator.db_file", "RECAC_DB_FILE")
	viper.BindEnv("orchestrator.max_iterations", "RECAC_MAX_ITERATIONS")
	viper.BindEnv("orchestrator.manager_frequency", "RECAC_MANAGER_FREQUENCY")
	viper.BindEnv("orchestrator.task_max_iterations", "RECAC_TASK_MAX_ITERATIONS")

	// Logger
	logger := telemetry.NewLogger(viper.GetBool("verbose"), "orchestrator", false)
	telemetry.InitLogger(viper.GetBool("verbose"), "orchestrator", false) // Ensure global logger is set

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, logger); err != nil {
		logger.Error("Application error", "error", err)
		exitFunc(1)
	}
}

func run(ctx context.Context, logger *slog.Logger) error {
	if viper.GetBool("orchestrator.list_jobs") {
		host := viper.GetString("orchestrator.host")
		history := viper.GetBool("orchestrator.history")
		listJobs(host, history)
		return nil
	}

	if logID := viper.GetString("orchestrator.logs"); logID != "" {
		host := viper.GetString("orchestrator.host")
		getLogs(host, logID)
		return nil
	}

	if jobID := viper.GetString("orchestrator.inspect_job"); jobID != "" {
		host := viper.GetString("orchestrator.host")
		inspectJob(host, jobID)
		return nil
	}

	if jobID := viper.GetString("orchestrator.cancel_job"); jobID != "" {
		host := viper.GetString("orchestrator.host")
		cancelJob(host, jobID)
		return nil
	}

	if jobID := viper.GetString("orchestrator.retry_job"); jobID != "" {
		host := viper.GetString("orchestrator.host")
		retryJob(host, jobID)
		return nil
	}

	if viper.GetBool("orchestrator.retry_failed") {
		host := viper.GetString("orchestrator.host")
		retryFailedJobs(host)
		return nil
	}

	if viper.GetBool("orchestrator.pause") {
		host := viper.GetString("orchestrator.host")
		pauseOrchestrator(host)
		return nil
	}

	if viper.GetBool("orchestrator.resume") {
		host := viper.GetString("orchestrator.host")
		resumeOrchestrator(host)
		return nil
	}

	if submitFile := viper.GetString("orchestrator.submit"); submitFile != "" {
		host := viper.GetString("orchestrator.host")
		wait := viper.GetBool("orchestrator.wait")
		submitJob(host, submitFile, wait)
		return nil
	}

	if submitURL := viper.GetString("orchestrator.submit_url"); submitURL != "" {
		host := viper.GetString("orchestrator.host")
		task := viper.GetString("orchestrator.submit_task")
		if task == "" {
			return fmt.Errorf("Error: --submit-task is required when using --submit-url")
		}
		id := viper.GetString("orchestrator.submit_id")
		wait := viper.GetBool("orchestrator.wait")
		submitAdHocJob(host, submitURL, task, id, wait)
		return nil
	}

	if viper.GetBool("orchestrator.monitor") {
		host := viper.GetString("orchestrator.host")
		if err := tui.StartDashboard(host); err != nil {
			return fmt.Errorf("Dashboard failed: %w", err)
		}
		return nil
	}

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
			return fmt.Errorf("Watch directory must be specified in file-dir poller mode")
		}
		var err error
		poller, err = orchestrator.NewFileDirPoller(watchDir)
		if err != nil {
			return fmt.Errorf("Failed to initialize file directory poller: %w", err)
		}
		logger.Info("Using file directory poller", "directory", watchDir)
	case "file", "filesystem":
		workFile := viper.GetString("orchestrator.work_file")
		if workFile == "" {
			return fmt.Errorf("Work file must be specified in file poller mode")
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
	case "gitlab":
		token := viper.GetString("orchestrator.gitlab_token")
		project := viper.GetString("orchestrator.gitlab_project")
		url := viper.GetString("orchestrator.gitlab_url")
		glLabel := viper.GetString("orchestrator.gitlab_label")
		if glLabel == "" {
			glLabel = label // Fallback to jira-label
		}

		if token == "" || project == "" {
			return fmt.Errorf("GitLab token and project must be specified in gitlab poller mode")
		}
		poller = orchestrator.NewGitLabPoller(url, token, project, glLabel)
		logger.Info("Using GitLab poller", "project", project, "label", glLabel)
	default:
		// Default to Jira
		jClient, err := cmdutils.GetJiraClient(ctx) // Use shared cmdutils
		if err != nil {
			return fmt.Errorf("Failed to initialize Jira client: %w", err)
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
	maxIterations := viper.GetInt("orchestrator.max_iterations")
	managerFrequency := viper.GetInt("orchestrator.manager_frequency")
	taskMaxIterations := viper.GetInt("orchestrator.task_max_iterations")

	// Initialize SessionManager early
	sm, err := runner.NewSessionManager()
	if err != nil {
		return fmt.Errorf("Failed to initialize Session Manager: %w", err)
	}

	var janitorClient orchestrator.DockerClient

	switch mode {
	case "k8s", "kubernetes":
		pullPolicy := corev1.PullPolicy(viper.GetString("orchestrator.image_pull_policy"))
		if pullPolicy == "" {
			pullPolicy = corev1.PullAlways
		}
		spawner, err = orchestrator.NewK8sSpawner(logger, image, namespace, poller, agentProvider, agentModel, pullPolicy, sm, maxIterations, managerFrequency, taskMaxIterations)
		if err != nil {
			return fmt.Errorf("Failed to initialize K8s spawner: %w", err)
		}
	case "local", "docker":
		projectName := "recac-orchestrator" // Or similar
		dockerCli, err := docker.NewClient(projectName)
		if err != nil {
			return fmt.Errorf("Failed to initialize Docker client: %w", err)
		}

		pullPolicy := viper.GetString("orchestrator.image_pull_policy")
		spawner = orchestrator.NewDockerSpawner(logger, dockerCli, image, projectName, poller, agentProvider, agentModel, pullPolicy, sm, maxIterations, managerFrequency, taskMaxIterations)
		janitorClient = dockerCli
	default:
		return fmt.Errorf("Invalid mode. Use 'local' or 'k8s': %s", mode)
	}

	// 3. Janitor
	if viper.GetBool("orchestrator.cleanup") && janitorClient != nil {
		janitor := orchestrator.NewJanitor(
			logger,
			janitorClient,
			viper.GetDuration("orchestrator.cleanup_interval"),
			viper.GetDuration("orchestrator.cleanup_age"),
			viper.GetBool("orchestrator.cleanup_dry_run"),
		)
		go janitor.Start(ctx)
	} else if viper.GetBool("orchestrator.cleanup") {
		logger.Warn("Cleanup enabled but not available in this mode (only local/docker)")
	}

	// 4. Orchestrator
	orch := orchestrator.New(poller, spawner, interval)

	// Persistence
	if dbPath := viper.GetString("orchestrator.db_file"); dbPath != "" {
		p := orchestrator.NewSQLitePersistence(dbPath)
		if err := p.Init(); err != nil {
			return fmt.Errorf("Failed to initialize persistence: %w", err)
		}
		defer p.Close()
		orch.SetPersistence(p)
		if err := orch.LoadHistory(logger); err != nil {
			logger.Error("Failed to load history", "error", err)
		} else {
			logger.Info("Persistence enabled", "db", dbPath)
		}
	}

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

	if viper.GetBool("orchestrator.verify") {
		if err := orch.Verify(ctx, logger); err != nil {
			logger.Error("Verification failed", "error", err)
			return fmt.Errorf("Verification failed: %w", err)
		}
		logger.Info("Verification passed successfully")
		return nil
	}

	if viper.GetBool("orchestrator.dry_run") {
		items, err := orch.DryRun(ctx, logger)
		if err != nil {
			logger.Error("Dry run failed", "error", err)
			return fmt.Errorf("Dry run failed: %w", err)
		}

		if len(items) == 0 {
			fmt.Println("No work items found.")
			return nil
		}

		var style = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(lipgloss.Color("#7D56F4")).
			PaddingTop(0).
			PaddingBottom(0).
			PaddingLeft(1).
			PaddingRight(1)

		fmt.Println(style.Render(fmt.Sprintf("Found %d work items:", len(items))))
		fmt.Println("")

		itemStyle := lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("63")).
			Padding(0, 1)

		titleStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("86")).Bold(true)
		descStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

		for _, item := range items {
			content := fmt.Sprintf("%s %s\n%s\nRepo: %s",
				titleStyle.Render(item.ID),
				item.Summary,
				descStyle.Render(limitString(item.Description, 100)),
				item.RepoURL)
			fmt.Println(itemStyle.Render(content))
		}
		return nil
	}

	if err := orch.Run(ctx, logger); err != nil {
		if ctx.Err() != nil {
			// Graceful shutdown
			return nil
		}
		logger.Error("Orchestrator failure", "error", err)
		return err
	}
	return nil
}

func listJobs(host string, history bool) {
	url := fmt.Sprintf("%s/jobs", host)
	if history {
		url += "?state=all"
	}

	resp, err := http.Get(url)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to connect to orchestrator at %s: %v\n", host, err)
		exitFunc(1)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(stdout, "Failed to fetch jobs: status %s\n", resp.Status)
		exitFunc(1)
		return
	}

	var jobs []orchestrator.JobInfo
	if err := json.NewDecoder(resp.Body).Decode(&jobs); err != nil {
		fmt.Fprintf(stdout, "Failed to decode response: %v\n", err)
		exitFunc(1)
		return
	}

	if len(jobs) == 0 {
		fmt.Fprintln(stdout, "No active jobs.")
		return
	}

	// Styles
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#7D56F4")).
		Padding(0, 1)

	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("252")).
		Padding(0, 1)

	rowStyle := lipgloss.NewStyle().
		Padding(0, 1)

	title := "Active Jobs"
	if history {
		title = "All Jobs (Active & History)"
	}
	fmt.Fprintln(stdout, titleStyle.Render(fmt.Sprintf("%s (%d)", title, len(jobs))))
	fmt.Fprintln(stdout, "")

	// Table Header
	fmt.Fprintf(stdout, "%-15s %-40s %-15s %-20s\n",
		headerStyle.Render("ID"),
		headerStyle.Render("Summary"),
		headerStyle.Render("Status"),
		headerStyle.Render("Duration"),
	)

	for _, job := range jobs {
		duration := time.Since(job.StartTime).Round(time.Second).String()
		fmt.Fprintf(stdout, "%-15s %-40s %-15s %-20s\n",
			rowStyle.Render(job.ID),
			rowStyle.Render(limitString(job.Summary, 38)),
			rowStyle.Render(job.Status),
			rowStyle.Render(duration),
		)
	}
}

func getLogs(host, jobID string) {
	resp, err := http.Get(fmt.Sprintf("%s/jobs/%s/logs", host, jobID))
	if err != nil {
		fmt.Fprintf(stdout, "Failed to connect to orchestrator at %s: %v\n", host, err)
		exitFunc(1)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(stdout, "Failed to fetch logs: %s\n", strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	// Stream logs to stdout
	if _, err := io.Copy(stdout, resp.Body); err != nil {
		fmt.Fprintf(stdout, "Failed to read logs: %v\n", err)
		exitFunc(1)
		return
	}
}

func limitString(s string, max int) string {
	if len(s) > max {
		return s[:max] + "..."
	}
	return s
}

func inspectJob(host, jobID string) {
	resp, err := http.Get(fmt.Sprintf("%s/jobs/%s", host, jobID))
	if err != nil {
		fmt.Fprintf(stdout, "Failed to connect to orchestrator at %s: %v\n", host, err)
		exitFunc(1)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(stdout, "Failed to fetch job details: %s\n", strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	var job orchestrator.JobInfo
	if err := json.NewDecoder(resp.Body).Decode(&job); err != nil {
		fmt.Fprintf(stdout, "Failed to decode response: %v\n", err)
		exitFunc(1)
		return
	}

	// Pretty print
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#7D56F4")).
		Padding(0, 1)

	labelStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("86")).
		Width(15)

	valueStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("252"))

	fmt.Fprintln(stdout, titleStyle.Render(fmt.Sprintf("Job Details: %s", job.ID)))
	fmt.Fprintln(stdout, "")

	printField := func(label, value string) {
		fmt.Fprintf(stdout, "%s %s\n", labelStyle.Render(label+":"), valueStyle.Render(value))
	}

	printField("Summary", job.Summary)
	printField("Status", job.Status)
	printField("Start Time", job.StartTime.Format(time.RFC3339))
	printField("Duration", time.Since(job.StartTime).Round(time.Second).String())
	fmt.Fprintln(stdout, "")
	printField("Repo URL", job.WorkItem.RepoURL)

	// Description
	fmt.Fprintln(stdout, labelStyle.Render("Description:"))
	fmt.Fprintln(stdout, lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render(job.WorkItem.Description))
	fmt.Fprintln(stdout, "")

	// Env Vars
	if len(job.WorkItem.EnvVars) > 0 {
		fmt.Fprintln(stdout, labelStyle.Render("Env Vars:"))
		for k, v := range job.WorkItem.EnvVars {
			// Mask likely secrets
			if strings.Contains(strings.ToLower(k), "token") || strings.Contains(strings.ToLower(k), "key") || strings.Contains(strings.ToLower(k), "secret") {
				v = "***"
			}
			fmt.Fprintf(stdout, "  %s=%s\n", k, v)
		}
	}
}

func cancelJob(host, jobID string) {
	req, err := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/jobs/%s", host, jobID), nil)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to create request: %v\n", err)
		exitFunc(1)
		return
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to connect to orchestrator at %s: %v\n", host, err)
		exitFunc(1)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(stdout, "Failed to cancel job: %s\n", strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	fmt.Fprintf(stdout, "Job %s cancelled successfully.\n", jobID)
}

func pauseOrchestrator(host string) {
	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/pause", host), nil)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to create request: %v\n", err)
		exitFunc(1)
		return
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to connect to orchestrator at %s: %v\n", host, err)
		exitFunc(1)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(stdout, "Failed to pause orchestrator: %s\n", strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	fmt.Fprintln(stdout, "Orchestrator paused.")
}

func resumeOrchestrator(host string) {
	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/resume", host), nil)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to create request: %v\n", err)
		exitFunc(1)
		return
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to connect to orchestrator at %s: %v\n", host, err)
		exitFunc(1)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(stdout, "Failed to resume orchestrator: %s\n", strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	fmt.Fprintln(stdout, "Orchestrator resumed.")
}

func retryJob(host, jobID string) {
	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/jobs/%s/retry", host, jobID), nil)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to create request: %v\n", err)
		exitFunc(1)
		return
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to connect to orchestrator at %s: %v\n", host, err)
		exitFunc(1)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(stdout, "Failed to retry job: %s\n", strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	fmt.Fprintf(stdout, "Job %s retry submitted successfully.\n", jobID)
}

func retryFailedJobs(host string) {
	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/jobs/retry-failed", host), nil)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to create request: %v\n", err)
		exitFunc(1)
		return
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to connect to orchestrator at %s: %v\n", host, err)
		exitFunc(1)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(stdout, "Failed to retry failed jobs: %s\n", strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	var result struct {
		Retried int `json:"retried"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		fmt.Fprintf(stdout, "Failed to decode response: %v\n", err)
		exitFunc(1)
		return
	}

	fmt.Fprintf(stdout, "Successfully retried %d failed jobs.\n", result.Retried)
}
