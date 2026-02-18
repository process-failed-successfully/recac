package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
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
	pflag.Bool("monitor", false, "Launch the TUI dashboard to monitor the orchestrator")
	pflag.String("logs", "", "Get logs for a specific job ID from a running orchestrator instance")
	pflag.String("inspect-job", "", "Inspect details of a specific job by ID")
	pflag.String("cancel-job", "", "Cancel a running job by ID")
	pflag.String("submit", "", "Submit a job from a JSON file path")
	pflag.String("host", "http://localhost:2112", "Orchestrator host URL (for list-jobs, logs, inspect-job, cancel-job, and submit)")

	pflag.String("mode", "local", "Orchestrator mode: 'local' (Docker) or 'k8s' (Kubernetes Job)")
	pflag.String("jira-label", "recac-agent", "Jira label to poll for")
	pflag.String("image", "ghcr.io/process-failed-successfully/recac-agent:latest", "Agent image to spawn")
	pflag.String("namespace", "default", "Kubernetes namespace (for k8s mode)")
	pflag.Duration("interval", 1*time.Minute, "Polling interval")
	pflag.String("agent-provider", "openrouter", "Provider for spawned agents")
	pflag.String("agent-model", "google/gemini-2.0-flash-001", "Model for spawned agents")
	pflag.String("image-pull-policy", "Always", "Image pull policy for agents (Always, IfNotPresent, Never)")
	pflag.Int("metrics-port", 2112, "Port to expose Prometheus metrics")

	// Janitor Flags
	pflag.Bool("cleanup", false, "Enable janitor to clean up old containers")
	pflag.Duration("cleanup-interval", 5*time.Minute, "Janitor check interval")
	pflag.Duration("cleanup-age", 24*time.Hour, "Age of containers to clean up")
	pflag.Bool("cleanup-dry-run", false, "Janitor dry run (log only)")

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

	viper.BindPFlag("orchestrator.dry_run", pflag.Lookup("dry-run"))
	viper.BindEnv("orchestrator.dry_run", "RECAC_ORCHESTRATOR_DRY_RUN")

	viper.BindPFlag("orchestrator.verify", pflag.Lookup("verify"))
	viper.BindPFlag("orchestrator.list_jobs", pflag.Lookup("list-jobs"))
	viper.BindPFlag("orchestrator.monitor", pflag.Lookup("monitor"))
	viper.BindPFlag("orchestrator.logs", pflag.Lookup("logs"))
	viper.BindPFlag("orchestrator.inspect_job", pflag.Lookup("inspect-job"))
	viper.BindPFlag("orchestrator.cancel_job", pflag.Lookup("cancel-job"))
	viper.BindPFlag("orchestrator.submit", pflag.Lookup("submit"))
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
	viper.BindEnv("orchestrator.mode", "RECAC_ORCHESTRATOR_MODE")
	viper.BindEnv("orchestrator.image", "RECAC_ORCHESTRATOR_IMAGE")
	viper.BindEnv("orchestrator.namespace", "RECAC_ORCHESTRATOR_NAMESPACE")
	viper.BindEnv("orchestrator.interval", "RECAC_ORCHESTRATOR_INTERVAL")
	viper.BindEnv("orchestrator.image_pull_policy", "RECAC_IMAGE_PULL_POLICY")
	viper.BindEnv("orchestrator.metrics_port", "RECAC_METRICS_PORT")
	viper.BindEnv("orchestrator.max_iterations", "RECAC_MAX_ITERATIONS")
	viper.BindEnv("orchestrator.manager_frequency", "RECAC_MANAGER_FREQUENCY")
	viper.BindEnv("orchestrator.task_max_iterations", "RECAC_TASK_MAX_ITERATIONS")

	// Logger
	logger := telemetry.NewLogger(viper.GetBool("verbose"), "orchestrator", false)
	telemetry.InitLogger(viper.GetBool("verbose"), "orchestrator", false) // Ensure global logger is set

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if viper.GetBool("orchestrator.list_jobs") {
		host := viper.GetString("orchestrator.host")
		listJobs(host)
		return
	}

	if logID := viper.GetString("orchestrator.logs"); logID != "" {
		host := viper.GetString("orchestrator.host")
		getLogs(host, logID)
		return
	}

	if inspectID := viper.GetString("orchestrator.inspect_job"); inspectID != "" {
		host := viper.GetString("orchestrator.host")
		inspectJob(host, inspectID)
		return
	}

	if jobID := viper.GetString("orchestrator.cancel_job"); jobID != "" {
		host := viper.GetString("orchestrator.host")
		cancelJob(host, jobID)
		return
	}

	if submitFile := viper.GetString("orchestrator.submit"); submitFile != "" {
		host := viper.GetString("orchestrator.host")
		submitJob(host, submitFile)
		return
	}

	if viper.GetBool("orchestrator.monitor") {
		host := viper.GetString("orchestrator.host")
		if err := tui.StartDashboard(host); err != nil {
			logger.Error("Dashboard failed", "error", err)
			os.Exit(1)
		}
		return
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
	var janitorClient orchestrator.DockerClient

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
		janitorClient = dockerCli
	default:
		logger.Error("Invalid mode. Use 'local' or 'k8s'", "mode", mode)
		os.Exit(1)
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

	// Start Metrics Server
	metricsPort := viper.GetInt("orchestrator.metrics_port")
	statusHandler := func(mux *http.ServeMux) {
		mux.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
			status := orch.GetStatus()
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(status); err != nil {
				logger.Error("Failed to encode status", "error", err)
			}
		})

		mux.HandleFunc("/jobs", func(w http.ResponseWriter, r *http.Request) {
			jobs := orch.GetActiveJobs()
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(jobs); err != nil {
				logger.Error("Failed to encode jobs", "error", err)
			}
		})

		mux.HandleFunc("GET /jobs/{id}/logs", func(w http.ResponseWriter, r *http.Request) {
			id := r.PathValue("id")
			logStream, err := orch.GetLogs(r.Context(), id)
			if err != nil {
				http.Error(w, err.Error(), http.StatusNotFound)
				return
			}
			defer logStream.Close()
			io.Copy(w, logStream)
		})

		mux.HandleFunc("GET /jobs/{id}", func(w http.ResponseWriter, r *http.Request) {
			id := r.PathValue("id")
			job, exists := orch.GetJob(id)
			if !exists {
				http.Error(w, "Job not found", http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(job); err != nil {
				logger.Error("Failed to encode job", "error", err)
			}
		})

		mux.HandleFunc("DELETE /jobs/{id}", func(w http.ResponseWriter, r *http.Request) {
			id := r.PathValue("id")
			if err := orch.CancelJob(r.Context(), id); err != nil {
				// We don't know if it's 404 or 500, but let's assume if it returns error, it failed.
				// For K8s "not found" it returns error too (in my impl).
				// Maybe parse error? Simplified for now.
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, "Job %s cancellation requested", id)
		})

		mux.HandleFunc("POST /jobs", func(w http.ResponseWriter, r *http.Request) {
			var item orchestrator.WorkItem
			if err := json.NewDecoder(r.Body).Decode(&item); err != nil {
				http.Error(w, "Invalid JSON body", http.StatusBadRequest)
				return
			}

			if item.ID == "" {
				http.Error(w, "Job ID is required", http.StatusBadRequest)
				return
			}

			// Use the context from main (captured via closure) to ensure job runs independently of the request context
			// but respects orchestrator shutdown.
			// Note: We are using the 'ctx' variable which is defined in main() and available here.
			if err := orch.SubmitJob(ctx, item, logger); err != nil {
				if strings.Contains(err.Error(), "already active") {
					http.Error(w, err.Error(), http.StatusConflict)
				} else {
					http.Error(w, err.Error(), http.StatusInternalServerError)
				}
				return
			}

			w.WriteHeader(http.StatusAccepted)
			fmt.Fprintf(w, "Job %s submitted successfully", item.ID)
		})
	}

	metricsServer, actualPort, err := telemetry.StartMetricsServer(metricsPort, statusHandler)
	if err != nil {
		logger.Error("Failed to start metrics server", "error", err)
	} else {
		logger.Info("Metrics server started", "port", actualPort)
		go func() {
			<-ctx.Done()
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
			os.Exit(1)
		}
		logger.Info("Verification passed successfully")
		return
	}

	if viper.GetBool("orchestrator.dry_run") {
		items, err := orch.DryRun(ctx, logger)
		if err != nil {
			logger.Error("Dry run failed", "error", err)
			os.Exit(1)
		}

		if len(items) == 0 {
			fmt.Println("No work items found.")
			return
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
		return
	}

	if err := orch.Run(ctx, logger); err != nil {
		if ctx.Err() != nil {
			// Graceful shutdown
			return
		}
		logger.Error("Orchestrator failure", "error", err)
		os.Exit(1)
	}
}

func listJobs(host string) {
	resp, err := http.Get(fmt.Sprintf("%s/jobs", host))
	if err != nil {
		fmt.Printf("Failed to connect to orchestrator at %s: %v\n", host, err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("Failed to fetch jobs: status %s\n", resp.Status)
		os.Exit(1)
	}

	var jobs []orchestrator.JobInfo
	if err := json.NewDecoder(resp.Body).Decode(&jobs); err != nil {
		fmt.Printf("Failed to decode response: %v\n", err)
		os.Exit(1)
	}

	if len(jobs) == 0 {
		fmt.Println("No active jobs.")
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

	fmt.Println(titleStyle.Render(fmt.Sprintf("Active Jobs (%d)", len(jobs))))
	fmt.Println("")

	// Table Header
	fmt.Printf("%-15s %-40s %-15s %-20s\n",
		headerStyle.Render("ID"),
		headerStyle.Render("Summary"),
		headerStyle.Render("Status"),
		headerStyle.Render("Duration"),
	)

	for _, job := range jobs {
		duration := time.Since(job.StartTime).Round(time.Second).String()
		fmt.Printf("%-15s %-40s %-15s %-20s\n",
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
		fmt.Printf("Failed to connect to orchestrator at %s: %v\n", host, err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Printf("Failed to fetch logs: %s\n", strings.TrimSpace(string(body)))
		os.Exit(1)
	}

	// Stream logs to stdout
	if _, err := io.Copy(os.Stdout, resp.Body); err != nil {
		fmt.Printf("Failed to read logs: %v\n", err)
		os.Exit(1)
	}
}

func limitString(s string, max int) string {
	if len(s) > max {
		return s[:max] + "..."
	}
	return s
}

func cancelJob(host, jobID string) {
	req, err := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/jobs/%s", host, jobID), nil)
	if err != nil {
		fmt.Printf("Failed to create request: %v\n", err)
		os.Exit(1)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Printf("Failed to connect to orchestrator at %s: %v\n", host, err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Printf("Failed to cancel job: %s\n", strings.TrimSpace(string(body)))
		os.Exit(1)
	}

	fmt.Printf("Job %s cancelled successfully.\n", jobID)
}

func submitJob(host, filePath string) {
	file, err := os.Open(filePath)
	if err != nil {
		fmt.Printf("Failed to open file %s: %v\n", filePath, err)
		os.Exit(1)
	}
	defer file.Close()

	// Verify JSON validity before sending (optional but good UX)
	var item map[string]interface{}
	if err := json.NewDecoder(file).Decode(&item); err != nil {
		fmt.Printf("Invalid JSON in file %s: %v\n", filePath, err)
		os.Exit(1)
	}
	// Reset file pointer
	file.Seek(0, 0)

	resp, err := http.Post(fmt.Sprintf("%s/jobs", host), "application/json", file)
	if err != nil {
		fmt.Printf("Failed to connect to orchestrator at %s: %v\n", host, err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusAccepted {
		fmt.Printf("Failed to submit job: %s\n", strings.TrimSpace(string(body)))
		os.Exit(1)
	}

	fmt.Printf("%s\n", strings.TrimSpace(string(body)))
}

func inspectJob(host, jobID string) {
	resp, err := http.Get(fmt.Sprintf("%s/jobs/%s", host, jobID))
	if err != nil {
		fmt.Printf("Failed to connect to orchestrator at %s: %v\n", host, err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Printf("Failed to fetch job details: %s\n", strings.TrimSpace(string(body)))
		os.Exit(1)
	}

	var job orchestrator.JobInfo
	if err := json.NewDecoder(resp.Body).Decode(&job); err != nil {
		fmt.Printf("Failed to decode response: %v\n", err)
		os.Exit(1)
	}

	// Pretty print using lipgloss
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#7D56F4")).
		Padding(0, 1).
		MarginBottom(1)

	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("252")).
		Bold(true).
		Width(15)

	valueStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("255"))

	fmt.Println(titleStyle.Render(fmt.Sprintf("Job Inspection: %s", job.ID)))

	printField := func(label, value string) {
		fmt.Printf("%s %s\n", labelStyle.Render(label), valueStyle.Render(value))
	}

	printField("Status", job.Status)
	printField("Summary", job.Summary)
	printField("Start Time", job.StartTime.Format(time.RFC1123))
	printField("Duration", time.Since(job.StartTime).Round(time.Second).String())

	fmt.Println("")
	fmt.Println(lipgloss.NewStyle().Foreground(lipgloss.Color("86")).Bold(true).Render("Work Item Details"))

	printField("Repo URL", job.WorkItem.RepoURL)
	if len(job.WorkItem.Description) > 0 {
		fmt.Println(labelStyle.Render("Description"))
		fmt.Println(job.WorkItem.Description)
	}

	if len(job.WorkItem.EnvVars) > 0 {
		fmt.Println("")
		fmt.Println(lipgloss.NewStyle().Foreground(lipgloss.Color("86")).Bold(true).Render("Environment Variables"))
		for k, v := range job.WorkItem.EnvVars {
			fmt.Printf("  %s=%s\n", k, v)
		}
	}
}
