package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"recac/internal/cmdutils"
	"recac/internal/config"
	"recac/internal/docker"
	"recac/internal/notify"
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
	pflag.Bool("list-pending", false, "List pending jobs from a running orchestrator instance")
	pflag.Bool("history", false, "Include completed jobs in list-jobs")
	pflag.String("list-jobs-status", "", "Filter jobs by status (e.g., Running, Failed, Completed)")
	pflag.String("list-jobs-tag", "", "Filter jobs by a specific tag")
	pflag.String("list-jobs-match", "", "Filter jobs by a regex matching the summary or error")
	pflag.String("list-jobs-format", "table", "Output format for list-jobs and list-pending (table, json)")
	pflag.Bool("status", false, "Get the current status of the orchestrator")
	pflag.Bool("tail-active", false, "Tail logs from all currently active jobs simultaneously")
	pflag.Bool("analytics", false, "Show orchestrator analytics")
	pflag.Bool("tree", false, "Display the dependency tree of jobs")
	pflag.Bool("monitor", false, "Launch the TUI dashboard to monitor the orchestrator")
	pflag.String("logs", "", "Get logs for a specific job ID from a running orchestrator instance")
	pflag.String("edit-job", "", "Edit a pending job interactively using $EDITOR")
	pflag.String("inspect-job", "", "Inspect a specific job by ID")
	pflag.String("cancel-job", "", "Cancel a running job by ID")
	pflag.Bool("cancel-all", false, "Cancel all currently running jobs")
	pflag.String("cancel-tag", "", "Cancel all active and pending jobs with the specified tag")
	pflag.String("cancel-status", "", "Cancel all active and pending jobs with the specified status")
	pflag.String("cancel-match", "", "Cancel all active and pending jobs matching the given regex")
	pflag.String("purge-job", "", "Purge a specific job from history")
	pflag.String("purge-tag", "", "Purge all completed/failed jobs with the specified tag from history")
	pflag.String("purge-status", "", "Purge all history jobs with the specified status")
	pflag.String("purge-match", "", "Purge all history jobs matching the given regex")
	pflag.Bool("purge-failed", false, "Purge all failed jobs from history")
	pflag.Bool("clear-history", false, "Clear all completed and failed jobs from history")
	pflag.Bool("clear-pending", false, "Clear all jobs waiting in the pending queue")
	pflag.String("retry-job", "", "Retry a completed job by ID")
	pflag.String("clone-job", "", "Clone an existing job by ID")
	pflag.Bool("retry-failed", false, "Retry all failed jobs from history")
	pflag.String("retry-match", "", "Optional regex to match against error messages when retrying failed jobs")
	pflag.String("retry-tag", "", "Retry all failed jobs from history with the specified tag")
	pflag.Bool("require-approval", false, "Require human approval before starting any job")
	pflag.String("approve-job", "", "Approve a job that is pending approval")
	pflag.String("hold-job", "", "Hold a pending job to prevent it from running")
	pflag.String("unhold-job", "", "Unhold a pending job to allow it to run")
	pflag.String("hold-tag", "", "Hold all pending jobs with the specified tag")
	pflag.String("hold-match", "", "Hold all pending jobs matching the given regex")
	pflag.String("unhold-tag", "", "Unhold all pending jobs with the specified tag")
	pflag.String("unhold-match", "", "Unhold all pending jobs matching the given regex")
	pflag.Bool("pause", false, "Pause the orchestrator polling loop")
	pflag.Bool("resume", false, "Resume the orchestrator polling loop")
	pflag.Bool("drain", false, "Set the orchestrator to drain mode")
	pflag.Bool("undrain", false, "Remove the orchestrator from drain mode")
	pflag.Bool("force-poll", false, "Force an immediate poll cycle")
	pflag.Int("scale", -1, "Dynamically scale the maximum concurrent jobs limit")
	pflag.String("update-priority", "", "Update the priority of a specific pending job")
	pflag.Int("priority-val", 0, "The new priority value to assign (requires --update-priority)")
	pflag.String("update-timeout", "", "Update the timeout of a specific pending job")
	pflag.String("timeout-val", "", "The new timeout value to assign (e.g., 30m) (requires --update-timeout)")
	pflag.String("set-progress-job", "", "Set progress for a specific job")
	pflag.Int("progress-val", -1, "The progress value to set (0-100) (requires --set-progress-job)")
	pflag.String("progress-msg", "", "Optional status message to set along with progress")
	pflag.String("update-deps-job", "", "Update the dependencies of a specific pending job")
	pflag.StringSlice("set-deps", []string{}, "Comma-separated list of new dependencies (requires --update-deps-job)")
	pflag.String("wait-job", "", "Wait for a specific job to complete and stream its logs")
	pflag.String("wait-tag", "", "Wait for all jobs with a specific tag to complete")
	pflag.String("set-output-job", "", "Set output key-value pair for a job")
	pflag.String("set-output-key", "", "Output key (requires --set-output-job)")
	pflag.String("set-output-val", "", "Output value (requires --set-output-job)")
	pflag.String("add-metrics-job", "", "Add metrics to a specific job")
	pflag.String("metrics-key", "", "The metrics key to add (requires --add-metrics-job)")
	pflag.Float64("metrics-val", 0, "The metrics value to add (requires --add-metrics-job)")
	pflag.String("submit", "", "Submit a job from a JSON file path")
	pflag.String("submit-batch", "", "Submit multiple jobs from a JSON file path")
	pflag.String("submit-matrix", "", "Submit a matrix job from a JSON file path")
	pflag.String("submit-pipeline", "", "Submit a pipeline job from a YAML file path")
	pflag.String("submit-url", "", "Repo URL for ad-hoc job submission")
	pflag.String("submit-task", "", "Task description for ad-hoc job submission")
	pflag.String("submit-id", "", "Optional ID for ad-hoc job submission")
	pflag.Int("submit-priority", 0, "Priority for ad-hoc job submission (higher is more important)")
	pflag.Duration("submit-delay", 0, "Delay before starting the ad-hoc job (e.g., 1m, 1h)")
	pflag.StringSlice("env", []string{}, "Environment variables to pass to the ad-hoc job (e.g., --env KEY=VALUE)")
	pflag.StringSlice("submit-deps", []string{}, "Comma-separated list of job IDs this job depends on")
	pflag.StringSlice("submit-tags", []string{}, "Comma-separated list of tags for the ad-hoc job")
	pflag.Duration("submit-timeout", 0, "Optional custom timeout for the ad-hoc job (e.g. 30m)")
	pflag.String("submit-concurrency-group", "", "Concurrency group for the ad-hoc job")
	pflag.Bool("submit-cancel-in-progress", false, "Cancel running jobs in the same concurrency group")
	pflag.String("submit-agent-provider", "", "Agent provider to use for the ad-hoc job")
	pflag.String("submit-agent-model", "", "Agent model to use for the ad-hoc job")
	pflag.Bool("wait", false, "Wait for job completion and stream logs (for submit/submit-url)")
	pflag.String("host", "http://localhost:2112", "Orchestrator host URL (for list-jobs, logs, cancel-job, and submit)")

	pflag.String("export-jobs", "", "Export all jobs to a file (use '-' for stdout)")
	pflag.String("import-jobs", "", "Import jobs from an exported JSON file")
	pflag.String("export-format", "json", "Format for exported jobs ('json' or 'csv')")

	pflag.String("mode", "local", "Orchestrator mode: 'local' (Docker), 'k8s' (Kubernetes Job), or 'process' (Local Process)")
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
	pflag.Int("max-concurrent-jobs", 0, "Maximum number of concurrent agent jobs allowed (0 = unlimited)")
	pflag.Duration("job-timeout", 0, "Maximum execution time for a job (0 = unlimited)")
	pflag.Int("max-retries", 0, "Maximum number of automatic retries for failed jobs")
	pflag.String("log-dir", "", "Directory to store persistent compressed job logs")
	pflag.Duration("retry-delay", 5*time.Second, "Delay between automatic retries")

	// Janitor Flags
	pflag.Bool("cleanup", false, "Enable janitor to clean up old containers")
	pflag.Duration("cleanup-interval", 5*time.Minute, "Janitor check interval")
	pflag.Duration("cleanup-age", 24*time.Hour, "Age of containers to clean up")
	pflag.Bool("cleanup-dry-run", false, "Janitor dry run (log only)")

	pflag.String("jira-query", "", "Custom JQL query (overrides label)")
	pflag.String("poller", "jira", "Poller type: 'jira', 'github', 'gitlab', 'file', 'file-dir', or 'cron'")
	pflag.String("work-file", "work_items.json", "Work items file (for 'file' poller)")
	pflag.String("watch-dir", "", "Directory to watch for work item files (for 'file-dir' poller)")

	pflag.String("cron-schedule", "", "Cron schedule expression (e.g. '0 0 * * *') (for 'cron' poller)")
	pflag.String("cron-template", "cron_template.json", "Path to WorkItem JSON template (for 'cron' poller)")

	pflag.String("github-token", "", "GitHub API Token (for 'github' poller)")
	pflag.String("github-owner", "", "GitHub Repository Owner (for 'github' poller)")
	pflag.String("github-repo", "", "GitHub Repository Name (for 'github' poller)")
	pflag.String("github-label", "", "GitHub Label to poll for (defaults to jira-label if not set)")
	pflag.String("github-webhook-secret", "", "GitHub Webhook Secret for validating incoming POST events")

	pflag.String("linear-token", "", "Linear API Token (for 'linear' poller)")
	pflag.String("linear-team", "", "Linear Team ID (for 'linear' poller)")
	pflag.String("linear-label", "", "Linear Label to poll for (for 'linear' poller)")

	pflag.String("gitlab-token", "", "GitLab API Token (for 'gitlab' poller)")
	pflag.String("gitlab-project", "", "GitLab Project ID or URL-encoded path (for 'gitlab' poller)")
	pflag.String("gitlab-label", "", "GitLab Label to poll for (defaults to jira-label if not set)")
	pflag.String("gitlab-url", "", "GitLab URL (defaults to https://gitlab.com)")
	pflag.String("gitlab-webhook-secret", "", "GitLab Webhook Secret for validating incoming POST events")

	pflag.String("trello-key", "", "Trello API Key (for 'trello' poller)")
	pflag.String("trello-token", "", "Trello API Token (for 'trello' poller)")
	pflag.String("trello-board", "", "Trello Board ID (for 'trello' poller)")
	pflag.String("trello-list", "", "Trello List ID to poll for (for 'trello' poller)")

	pflag.String("asana-token", "", "Asana API Token (for 'asana' poller)")
	pflag.String("asana-project", "", "Asana Project ID (for 'asana' poller)")

	pflag.String("notion-token", "", "Notion API Token (for 'notion' poller)")
	pflag.String("notion-database-id", "", "Notion Database ID (for 'notion' poller)")
	pflag.String("notion-label", "", "Notion Label/Tag to poll for (defaults to jira-label if not set)")

	pflag.Bool("webhook-enabled", false, "Enable generic webhook notifications")
	pflag.String("webhook-url", "", "URL for generic webhook notifications")
	pflag.String("webhook-secret", "", "Secret for generic webhook HMAC signature")

	pflag.Parse()

	// Config
	config.Load(cfgFile)

	// Bind Flags
	viper.BindPFlag("verbose", pflag.Lookup("verbose"))
	viper.BindPFlag("orchestrator.jira_query", pflag.Lookup("jira-query"))
	viper.BindPFlag("orchestrator.poller", pflag.Lookup("poller"))
	viper.BindPFlag("orchestrator.work_file", pflag.Lookup("work-file"))
	viper.BindPFlag("orchestrator.watch_dir", pflag.Lookup("watch-dir"))

	viper.BindPFlag("orchestrator.cron_schedule", pflag.Lookup("cron-schedule"))
	viper.BindPFlag("orchestrator.cron_template", pflag.Lookup("cron-template"))

	viper.BindPFlag("orchestrator.github_token", pflag.Lookup("github-token"))
	viper.BindPFlag("orchestrator.github_owner", pflag.Lookup("github-owner"))
	viper.BindPFlag("orchestrator.github_repo", pflag.Lookup("github-repo"))
	viper.BindPFlag("orchestrator.github_label", pflag.Lookup("github-label"))
	viper.BindPFlag("orchestrator.github_webhook_secret", pflag.Lookup("github-webhook-secret"))

	viper.BindPFlag("orchestrator.linear_token", pflag.Lookup("linear-token"))
	viper.BindPFlag("orchestrator.linear_team", pflag.Lookup("linear-team"))
	viper.BindPFlag("orchestrator.linear_label", pflag.Lookup("linear-label"))

	viper.BindPFlag("orchestrator.gitlab_token", pflag.Lookup("gitlab-token"))
	viper.BindPFlag("orchestrator.gitlab_project", pflag.Lookup("gitlab-project"))
	viper.BindPFlag("orchestrator.gitlab_label", pflag.Lookup("gitlab-label"))
	viper.BindPFlag("orchestrator.gitlab_url", pflag.Lookup("gitlab-url"))
	viper.BindPFlag("orchestrator.gitlab_webhook_secret", pflag.Lookup("gitlab-webhook-secret"))

	viper.BindPFlag("orchestrator.dry_run", pflag.Lookup("dry-run"))
	viper.BindEnv("orchestrator.dry_run", "RECAC_ORCHESTRATOR_DRY_RUN")

	viper.BindPFlag("orchestrator.verify", pflag.Lookup("verify"))
	viper.BindPFlag("orchestrator.list_jobs", pflag.Lookup("list-jobs"))
	viper.BindPFlag("orchestrator.list_pending", pflag.Lookup("list-pending"))
	viper.BindPFlag("orchestrator.tree", pflag.Lookup("tree"))
	viper.BindPFlag("orchestrator.history", pflag.Lookup("history"))
	viper.BindPFlag("orchestrator.list_jobs_status", pflag.Lookup("list-jobs-status"))
	viper.BindPFlag("orchestrator.list_jobs_tag", pflag.Lookup("list-jobs-tag"))
	viper.BindPFlag("orchestrator.list_jobs_match", pflag.Lookup("list-jobs-match"))
	viper.BindPFlag("orchestrator.list_jobs_format", pflag.Lookup("list-jobs-format"))
	viper.BindPFlag("orchestrator.status", pflag.Lookup("status"))
	viper.BindPFlag("orchestrator.tail_active", pflag.Lookup("tail-active"))
	viper.BindPFlag("orchestrator.analytics", pflag.Lookup("analytics"))
	viper.BindPFlag("orchestrator.monitor", pflag.Lookup("monitor"))
	viper.BindPFlag("orchestrator.logs", pflag.Lookup("logs"))
	viper.BindPFlag("orchestrator.edit_job", pflag.Lookup("edit-job"))
	viper.BindPFlag("orchestrator.inspect_job", pflag.Lookup("inspect-job"))
	viper.BindPFlag("orchestrator.cancel_job", pflag.Lookup("cancel-job"))
	viper.BindPFlag("orchestrator.cancel_all", pflag.Lookup("cancel-all"))
	viper.BindPFlag("orchestrator.cancel_tag", pflag.Lookup("cancel-tag"))
	viper.BindPFlag("orchestrator.cancel_status", pflag.Lookup("cancel-status"))
	viper.BindPFlag("orchestrator.cancel_match", pflag.Lookup("cancel-match"))
	viper.BindPFlag("orchestrator.purge_job", pflag.Lookup("purge-job"))
	viper.BindPFlag("orchestrator.purge_tag", pflag.Lookup("purge-tag"))
	viper.BindPFlag("orchestrator.purge_status", pflag.Lookup("purge-status"))
	viper.BindPFlag("orchestrator.purge_match", pflag.Lookup("purge-match"))
	viper.BindPFlag("orchestrator.purge_failed", pflag.Lookup("purge-failed"))
	viper.BindPFlag("orchestrator.clear_history", pflag.Lookup("clear-history"))
	viper.BindPFlag("orchestrator.clear_pending", pflag.Lookup("clear-pending"))
	viper.BindPFlag("orchestrator.retry_job", pflag.Lookup("retry-job"))
	viper.BindPFlag("orchestrator.clone_job", pflag.Lookup("clone-job"))
	viper.BindPFlag("orchestrator.retry_failed", pflag.Lookup("retry-failed"))
	viper.BindPFlag("orchestrator.retry_match", pflag.Lookup("retry-match"))
	viper.BindPFlag("orchestrator.retry_tag", pflag.Lookup("retry-tag"))
	viper.BindPFlag("orchestrator.require_approval", pflag.Lookup("require-approval"))
	viper.BindPFlag("orchestrator.approve_job", pflag.Lookup("approve-job"))
	viper.BindPFlag("orchestrator.hold_job", pflag.Lookup("hold-job"))
	viper.BindPFlag("orchestrator.unhold_job", pflag.Lookup("unhold-job"))
	viper.BindPFlag("orchestrator.hold_tag", pflag.Lookup("hold-tag"))
	viper.BindPFlag("orchestrator.hold_match", pflag.Lookup("hold-match"))
	viper.BindPFlag("orchestrator.unhold_tag", pflag.Lookup("unhold-tag"))
	viper.BindPFlag("orchestrator.unhold_match", pflag.Lookup("unhold-match"))
	viper.BindPFlag("orchestrator.pause", pflag.Lookup("pause"))
	viper.BindPFlag("orchestrator.resume", pflag.Lookup("resume"))
	viper.BindPFlag("orchestrator.drain", pflag.Lookup("drain"))
	viper.BindPFlag("orchestrator.undrain", pflag.Lookup("undrain"))
	viper.BindPFlag("orchestrator.force_poll", pflag.Lookup("force-poll"))
	viper.BindPFlag("orchestrator.scale", pflag.Lookup("scale"))
	viper.BindPFlag("orchestrator.update_priority", pflag.Lookup("update-priority"))
	viper.BindPFlag("orchestrator.priority_val", pflag.Lookup("priority-val"))
	viper.BindPFlag("orchestrator.update_timeout", pflag.Lookup("update-timeout"))
	viper.BindPFlag("orchestrator.timeout_val", pflag.Lookup("timeout-val"))
	viper.BindPFlag("orchestrator.set_progress_job", pflag.Lookup("set-progress-job"))
	viper.BindPFlag("orchestrator.progress_val", pflag.Lookup("progress-val"))
	viper.BindPFlag("orchestrator.progress_msg", pflag.Lookup("progress-msg"))
	viper.BindPFlag("orchestrator.update_deps_job", pflag.Lookup("update-deps-job"))
	viper.BindPFlag("orchestrator.set_deps", pflag.Lookup("set-deps"))
	viper.BindPFlag("orchestrator.wait_job", pflag.Lookup("wait-job"))
	viper.BindPFlag("orchestrator.wait_tag", pflag.Lookup("wait-tag"))
	viper.BindPFlag("orchestrator.set_output_job", pflag.Lookup("set-output-job"))
	viper.BindPFlag("orchestrator.set_output_key", pflag.Lookup("set-output-key"))
	viper.BindPFlag("orchestrator.set_output_val", pflag.Lookup("set-output-val"))
	viper.BindPFlag("orchestrator.add_metrics_job", pflag.Lookup("add-metrics-job"))
	viper.BindPFlag("orchestrator.metrics_key", pflag.Lookup("metrics-key"))
	viper.BindPFlag("orchestrator.metrics_val", pflag.Lookup("metrics-val"))
	viper.BindPFlag("orchestrator.submit", pflag.Lookup("submit"))
	viper.BindPFlag("orchestrator.submit_batch", pflag.Lookup("submit-batch"))
	viper.BindPFlag("orchestrator.submit_matrix", pflag.Lookup("submit-matrix"))
	viper.BindPFlag("orchestrator.submit_pipeline", pflag.Lookup("submit-pipeline"))
	viper.BindPFlag("orchestrator.submit_url", pflag.Lookup("submit-url"))
	viper.BindPFlag("orchestrator.submit_task", pflag.Lookup("submit-task"))
	viper.BindPFlag("orchestrator.submit_id", pflag.Lookup("submit-id"))
	viper.BindPFlag("orchestrator.submit_priority", pflag.Lookup("submit-priority"))
	viper.BindPFlag("orchestrator.submit_delay", pflag.Lookup("submit-delay"))
	viper.BindPFlag("orchestrator.env", pflag.Lookup("env"))
	viper.BindPFlag("orchestrator.submit_deps", pflag.Lookup("submit-deps"))
	viper.BindPFlag("orchestrator.submit_tags", pflag.Lookup("submit-tags"))
	viper.BindPFlag("orchestrator.submit_timeout", pflag.Lookup("submit-timeout"))
	viper.BindPFlag("orchestrator.submit_concurrency_group", pflag.Lookup("submit-concurrency-group"))
	viper.BindPFlag("orchestrator.submit_cancel_in_progress", pflag.Lookup("submit-cancel-in-progress"))
	viper.BindPFlag("orchestrator.submit_agent_provider", pflag.Lookup("submit-agent-provider"))
	viper.BindPFlag("orchestrator.submit_agent_model", pflag.Lookup("submit-agent-model"))
	viper.BindPFlag("orchestrator.wait", pflag.Lookup("wait"))
	viper.BindPFlag("orchestrator.host", pflag.Lookup("host"))

	viper.BindPFlag("orchestrator.export_jobs", pflag.Lookup("export-jobs"))
	viper.BindPFlag("orchestrator.import_jobs", pflag.Lookup("import-jobs"))
	viper.BindPFlag("orchestrator.export_format", pflag.Lookup("export-format"))

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
	viper.BindPFlag("orchestrator.max_concurrent_jobs", pflag.Lookup("max-concurrent-jobs"))
	viper.BindPFlag("orchestrator.job_timeout", pflag.Lookup("job-timeout"))
	viper.BindPFlag("orchestrator.max_retries", pflag.Lookup("max-retries"))
	viper.BindPFlag("orchestrator.log_dir", pflag.Lookup("log-dir"))
	viper.BindPFlag("orchestrator.retry_delay", pflag.Lookup("retry-delay"))

	viper.BindPFlag("orchestrator.cleanup", pflag.Lookup("cleanup"))
	viper.BindPFlag("orchestrator.cleanup_interval", pflag.Lookup("cleanup-interval"))
	viper.BindPFlag("orchestrator.cleanup_age", pflag.Lookup("cleanup-age"))

	viper.BindPFlag("orchestrator.trello_key", pflag.Lookup("trello-key"))
	viper.BindPFlag("orchestrator.trello_token", pflag.Lookup("trello-token"))
	viper.BindPFlag("orchestrator.trello_board", pflag.Lookup("trello-board"))
	viper.BindPFlag("orchestrator.trello_list", pflag.Lookup("trello-list"))

	viper.BindPFlag("orchestrator.asana_token", pflag.Lookup("asana-token"))
	viper.BindPFlag("orchestrator.asana_project", pflag.Lookup("asana-project"))

	viper.BindPFlag("orchestrator.notion_token", pflag.Lookup("notion-token"))
	viper.BindPFlag("orchestrator.notion_database_id", pflag.Lookup("notion-database-id"))
	viper.BindPFlag("orchestrator.notion_label", pflag.Lookup("notion-label"))

	viper.BindPFlag("orchestrator.cleanup_dry_run", pflag.Lookup("cleanup-dry-run"))

	viper.BindPFlag("notifications.webhook.enabled", pflag.Lookup("webhook-enabled"))
	viper.BindPFlag("notifications.webhook.url", pflag.Lookup("webhook-url"))
	viper.BindPFlag("notifications.webhook.secret", pflag.Lookup("webhook-secret"))

	// Explicitly bind cleaner env vars
	viper.BindEnv("orchestrator.agent_provider", "RECAC_AGENT_PROVIDER")
	viper.BindEnv("orchestrator.agent_model", "RECAC_AGENT_MODEL")
	viper.BindEnv("orchestrator.poller", "RECAC_POLLER")
	viper.BindEnv("orchestrator.work_file", "RECAC_WORK_FILE")
	viper.BindEnv("orchestrator.watch_dir", "RECAC_WATCH_DIR")
	viper.BindEnv("orchestrator.cron_schedule", "RECAC_CRON_SCHEDULE")
	viper.BindEnv("orchestrator.cron_template", "RECAC_CRON_TEMPLATE")
	viper.BindEnv("orchestrator.github_token", "RECAC_GITHUB_TOKEN", "GITHUB_TOKEN")
	viper.BindEnv("orchestrator.github_owner", "RECAC_GITHUB_OWNER")
	viper.BindEnv("orchestrator.github_repo", "RECAC_GITHUB_REPO")
	viper.BindEnv("orchestrator.github_label", "RECAC_GITHUB_LABEL")
	viper.BindEnv("orchestrator.github_webhook_secret", "RECAC_GITHUB_WEBHOOK_SECRET")
	viper.BindEnv("orchestrator.linear_token", "RECAC_LINEAR_TOKEN", "LINEAR_TOKEN")
	viper.BindEnv("orchestrator.linear_team", "RECAC_LINEAR_TEAM")
	viper.BindEnv("orchestrator.linear_label", "RECAC_LINEAR_LABEL")
	viper.BindEnv("orchestrator.gitlab_token", "RECAC_GITLAB_TOKEN", "GITLAB_TOKEN")
	viper.BindEnv("orchestrator.gitlab_project", "RECAC_GITLAB_PROJECT")
	viper.BindEnv("orchestrator.gitlab_label", "RECAC_GITLAB_LABEL")
	viper.BindEnv("orchestrator.gitlab_url", "RECAC_GITLAB_URL")
	viper.BindEnv("orchestrator.gitlab_webhook_secret", "RECAC_GITLAB_WEBHOOK_SECRET")
	viper.BindEnv("orchestrator.trello_key", "RECAC_TRELLO_KEY")
	viper.BindEnv("orchestrator.trello_token", "RECAC_TRELLO_TOKEN")
	viper.BindEnv("orchestrator.trello_board", "RECAC_TRELLO_BOARD")
	viper.BindEnv("orchestrator.trello_list", "RECAC_TRELLO_LIST")
	viper.BindEnv("orchestrator.asana_token", "RECAC_ASANA_TOKEN", "ASANA_TOKEN")
	viper.BindEnv("orchestrator.asana_project", "RECAC_ASANA_PROJECT")
	viper.BindEnv("orchestrator.notion_token", "RECAC_NOTION_TOKEN", "NOTION_TOKEN")
	viper.BindEnv("orchestrator.notion_database_id", "RECAC_NOTION_DATABASE_ID", "NOTION_DATABASE_ID")
	viper.BindEnv("orchestrator.notion_label", "RECAC_NOTION_LABEL", "NOTION_LABEL")
	viper.BindEnv("notifications.webhook.enabled", "RECAC_WEBHOOK_ENABLED")
	viper.BindEnv("notifications.webhook.url", "RECAC_WEBHOOK_URL")
	viper.BindEnv("notifications.webhook.secret", "RECAC_WEBHOOK_SECRET")
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
	viper.BindEnv("orchestrator.max_concurrent_jobs", "RECAC_MAX_CONCURRENT_JOBS")
	viper.BindEnv("orchestrator.job_timeout", "RECAC_JOB_TIMEOUT")
	viper.BindEnv("orchestrator.max_retries", "RECAC_MAX_RETRIES")
	viper.BindEnv("orchestrator.log_dir", "RECAC_LOG_DIR")
	viper.BindEnv("orchestrator.retry_delay", "RECAC_RETRY_DELAY")
	viper.BindEnv("orchestrator.require_approval", "RECAC_REQUIRE_APPROVAL")

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
		statusFilter := viper.GetString("orchestrator.list_jobs_status")
		tagFilter := viper.GetString("orchestrator.list_jobs_tag")
		matchFilter := viper.GetString("orchestrator.list_jobs_match")
		format := viper.GetString("orchestrator.list_jobs_format")
		listJobs(host, history, statusFilter, tagFilter, matchFilter, format)
		return nil
	}

	if viper.GetBool("orchestrator.list_pending") {
		host := viper.GetString("orchestrator.host")
		format := viper.GetString("orchestrator.list_jobs_format")
		listPendingJobs(host, format)
		return nil
	}

	if viper.GetBool("orchestrator.status") {
		host := viper.GetString("orchestrator.host")
		printStatus(host)
		return nil
	}

	if viper.GetBool("orchestrator.tail_active") {
		host := viper.GetString("orchestrator.host")
		if err := tailActiveJobs(ctx, host); err != nil {
			fmt.Fprintf(stdout, "Tail failed: %v\n", err)
			exitFunc(1)
		}
		return nil
	}

	if outputJob := viper.GetString("orchestrator.set_output_job"); outputJob != "" {
		host := viper.GetString("orchestrator.host")
		key := viper.GetString("orchestrator.set_output_key")
		val := viper.GetString("orchestrator.set_output_val")
		if key == "" {
			fmt.Fprintf(stdout, "Error: --set-output-key is required when using --set-output-job\n")
			exitFunc(1)
			return nil
		}
		setJobOutput(host, outputJob, key, val)
		return nil
	}

	if addMetricsJob := viper.GetString("orchestrator.add_metrics_job"); addMetricsJob != "" {
		host := viper.GetString("orchestrator.host")
		key := viper.GetString("orchestrator.metrics_key")
		val := viper.GetFloat64("orchestrator.metrics_val")
		if key == "" {
			fmt.Fprintf(stdout, "Error: --metrics-key is required when using --add-metrics-job\n")
			exitFunc(1)
			return nil
		}
		addJobMetrics(host, addMetricsJob, key, val)
		return nil
	}

	if viper.GetBool("orchestrator.analytics") {
		host := viper.GetString("orchestrator.host")
		printAnalytics(host)
		return nil
	}

	if viper.GetBool("orchestrator.tree") {
		host := viper.GetString("orchestrator.host")
		printTree(host)
		return nil
	}

	if logID := viper.GetString("orchestrator.logs"); logID != "" {
		host := viper.GetString("orchestrator.host")
		getLogs(host, logID)
		return nil
	}

	if jobID := viper.GetString("orchestrator.edit_job"); jobID != "" {
		host := viper.GetString("orchestrator.host")
		editJob(host, jobID)
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

	if viper.GetBool("orchestrator.cancel_all") {
		host := viper.GetString("orchestrator.host")
		cancelAllJobs(host)
		return nil
	}

	if cancelTag := viper.GetString("orchestrator.cancel_tag"); cancelTag != "" {
		host := viper.GetString("orchestrator.host")
		cancelJobsByTag(host, cancelTag)
		return nil
	}

	if cancelStatus := viper.GetString("orchestrator.cancel_status"); cancelStatus != "" {
		host := viper.GetString("orchestrator.host")
		cancelJobsByStatus(host, cancelStatus)
		return nil
	}

	if cancelMatch := viper.GetString("orchestrator.cancel_match"); cancelMatch != "" {
		host := viper.GetString("orchestrator.host")
		cancelJobsByMatch(host, cancelMatch)
		return nil
	}

	if jobID := viper.GetString("orchestrator.purge_job"); jobID != "" {
		host := viper.GetString("orchestrator.host")
		purgeJob(host, jobID)
		return nil
	}

	if purgeTag := viper.GetString("orchestrator.purge_tag"); purgeTag != "" {
		host := viper.GetString("orchestrator.host")
		purgeJobsByTag(host, purgeTag)
		return nil
	}

	if purgeStatus := viper.GetString("orchestrator.purge_status"); purgeStatus != "" {
		host := viper.GetString("orchestrator.host")
		purgeJobsByStatus(host, purgeStatus)
		return nil
	}

	if purgeMatch := viper.GetString("orchestrator.purge_match"); purgeMatch != "" {
		host := viper.GetString("orchestrator.host")
		purgeJobsByMatch(host, purgeMatch)
		return nil
	}

	if viper.GetBool("orchestrator.purge_failed") {
		host := viper.GetString("orchestrator.host")
		purgeJobsByStatus(host, "Failed")
		return nil
	}

	if viper.GetBool("orchestrator.clear_history") {
		host := viper.GetString("orchestrator.host")
		clearHistory(host)
		return nil
	}

	if viper.GetBool("orchestrator.clear_pending") {
		host := viper.GetString("orchestrator.host")
		clearPending(host)
		return nil
	}

	if jobID := viper.GetString("orchestrator.retry_job"); jobID != "" {
		host := viper.GetString("orchestrator.host")
		retryJob(host, jobID)
		return nil
	}

	if jobID := viper.GetString("orchestrator.clone_job"); jobID != "" {
		host := viper.GetString("orchestrator.host")
		id := viper.GetString("orchestrator.submit_id")
		priority := viper.GetInt("orchestrator.submit_priority")
		wait := viper.GetBool("orchestrator.wait")
		envPairs := viper.GetStringSlice("orchestrator.env")
		submitDeps := viper.GetStringSlice("orchestrator.submit_deps")

		envMap := make(map[string]string)
		for _, pair := range envPairs {
			parts := strings.SplitN(pair, "=", 2)
			if len(parts) == 2 {
				envMap[parts[0]] = parts[1]
			} else {
				logger.Warn("Invalid environment variable format", "input", pair)
			}
		}

		priorityPtr := &priority
		if !viper.IsSet("orchestrator.submit_priority") {
			priorityPtr = nil
		}

		var submitDepsPtr []string
		if viper.IsSet("orchestrator.submit_deps") {
			submitDepsPtr = submitDeps
		}

		cloneJob(host, jobID, id, priorityPtr, wait, envMap, submitDepsPtr)
		return nil
	}

	retryMatch := viper.GetString("orchestrator.retry_match")
	retryTag := viper.GetString("orchestrator.retry_tag")
	if viper.GetBool("orchestrator.retry_failed") || retryMatch != "" || retryTag != "" {
		host := viper.GetString("orchestrator.host")
		retryFailedJobs(host, retryMatch, retryTag)
		return nil
	}

	if approveJobId := viper.GetString("orchestrator.approve_job"); approveJobId != "" {
		host := viper.GetString("orchestrator.host")
		approveJob(host, approveJobId)
		return nil
	}

	if holdJobID := viper.GetString("orchestrator.hold_job"); holdJobID != "" {
		host := viper.GetString("orchestrator.host")
		holdJob(host, holdJobID)
		return nil
	}

	holdTag := viper.GetString("orchestrator.hold_tag")
	holdMatch := viper.GetString("orchestrator.hold_match")
	if holdTag != "" || holdMatch != "" {
		host := viper.GetString("orchestrator.host")
		holdJobs(host, holdMatch, holdTag)
		return nil
	}

	if unholdJobID := viper.GetString("orchestrator.unhold_job"); unholdJobID != "" {
		host := viper.GetString("orchestrator.host")
		unholdJob(host, unholdJobID)
		return nil
	}

	unholdTag := viper.GetString("orchestrator.unhold_tag")
	unholdMatch := viper.GetString("orchestrator.unhold_match")
	if unholdTag != "" || unholdMatch != "" {
		host := viper.GetString("orchestrator.host")
		unholdJobs(host, unholdMatch, unholdTag)
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

	if viper.GetBool("orchestrator.drain") {
		host := viper.GetString("orchestrator.host")
		drainOrchestrator(host)
		return nil
	}

	if viper.GetBool("orchestrator.undrain") {
		host := viper.GetString("orchestrator.host")
		undrainOrchestrator(host)
		return nil
	}

	if viper.GetBool("orchestrator.force_poll") {
		host := viper.GetString("orchestrator.host")
		forcePoll(host)
		return nil
	}

	if scaleVal := viper.GetInt("orchestrator.scale"); scaleVal >= 0 {
		host := viper.GetString("orchestrator.host")
		scaleConcurrency(host, scaleVal)
		return nil
	}

	if updateJob := viper.GetString("orchestrator.update_priority"); updateJob != "" {
		host := viper.GetString("orchestrator.host")
		priorityVal := viper.GetInt("orchestrator.priority_val")
		updatePriority(host, updateJob, priorityVal)
		return nil
	}

	if updateTimeoutJob := viper.GetString("orchestrator.update_timeout"); updateTimeoutJob != "" {
		host := viper.GetString("orchestrator.host")
		timeoutVal := viper.GetString("orchestrator.timeout_val")
		if timeoutVal == "" {
			fmt.Fprintf(stdout, "Error: --timeout-val is required when using --update-timeout\n")
			exitFunc(1)
			return nil
		}
		updateTimeout(host, updateTimeoutJob, timeoutVal)
		return nil
	}

	if setProgressJob := viper.GetString("orchestrator.set_progress_job"); setProgressJob != "" {
		host := viper.GetString("orchestrator.host")
		progressVal := viper.GetInt("orchestrator.progress_val")
		progressMsg := viper.GetString("orchestrator.progress_msg")

		var pVal *int
		if progressVal >= 0 && progressVal <= 100 {
			pVal = &progressVal
		}

		var pMsg *string
		if progressMsg != "" {
			pMsg = &progressMsg
		}

		if pVal == nil && pMsg == nil {
			fmt.Fprintf(stdout, "Error: Must provide either a valid --progress-val (0-100) or a --progress-msg\n")
			exitFunc(1)
			return nil
		}

		setJobProgress(host, setProgressJob, pVal, pMsg)
		return nil
	}

	if updateDepsJob := viper.GetString("orchestrator.update_deps_job"); updateDepsJob != "" {
		host := viper.GetString("orchestrator.host")
		var setDepsPtr []string
		if viper.IsSet("orchestrator.set_deps") {
			setDepsPtr = viper.GetStringSlice("orchestrator.set_deps")
		} else {
			setDepsPtr = []string{}
		}
		updateDependencies(host, updateDepsJob, setDepsPtr)
		return nil
	}

	if waitJob := viper.GetString("orchestrator.wait_job"); waitJob != "" {
		host := viper.GetString("orchestrator.host")
		if err := waitForJob(host, waitJob, stdout); err != nil {
			fmt.Fprintf(stdout, "Job failed: %v\n", err)
			exitFunc(1)
		}
		return nil
	}

	if waitTag := viper.GetString("orchestrator.wait_tag"); waitTag != "" {
		host := viper.GetString("orchestrator.host")
		if err := waitForTag(host, waitTag, stdout); err != nil {
			fmt.Fprintf(stdout, "Tag %s wait failed: %v\n", waitTag, err)
			exitFunc(1)
		}
		return nil
	}

	if submitFile := viper.GetString("orchestrator.submit"); submitFile != "" {
		host := viper.GetString("orchestrator.host")
		wait := viper.GetBool("orchestrator.wait")
		submitJob(host, submitFile, wait)
		return nil
	}

	if submitBatchFile := viper.GetString("orchestrator.submit_batch"); submitBatchFile != "" {
		host := viper.GetString("orchestrator.host")
		wait := viper.GetBool("orchestrator.wait")
		submitBatchJob(host, submitBatchFile, wait)
		return nil
	}

	if submitMatrixFile := viper.GetString("orchestrator.submit_matrix"); submitMatrixFile != "" {
		host := viper.GetString("orchestrator.host")
		wait := viper.GetBool("orchestrator.wait")
		submitMatrixJob(host, submitMatrixFile, wait)
		return nil
	}

	if submitPipelineFile := viper.GetString("orchestrator.submit_pipeline"); submitPipelineFile != "" {
		host := viper.GetString("orchestrator.host")
		wait := viper.GetBool("orchestrator.wait")
		submitPipelineJob(host, submitPipelineFile, wait)
		return nil
	}

	if submitURL := viper.GetString("orchestrator.submit_url"); submitURL != "" {
		host := viper.GetString("orchestrator.host")
		task := viper.GetString("orchestrator.submit_task")
		if task == "" {
			return fmt.Errorf("Error: --submit-task is required when using --submit-url")
		}
		id := viper.GetString("orchestrator.submit_id")
		priority := viper.GetInt("orchestrator.submit_priority")
		wait := viper.GetBool("orchestrator.wait")
		envPairs := viper.GetStringSlice("orchestrator.env")
		submitDeps := viper.GetStringSlice("orchestrator.submit_deps")

		envMap := make(map[string]string)
		for _, pair := range envPairs {
			parts := strings.SplitN(pair, "=", 2)
			if len(parts) == 2 {
				envMap[parts[0]] = parts[1]
			} else {
				logger.Warn("Invalid environment variable format", "input", pair)
			}
		}

		delay := viper.GetDuration("orchestrator.submit_delay")
		timeout := viper.GetDuration("orchestrator.submit_timeout")
		submitTags := viper.GetStringSlice("orchestrator.submit_tags")
		concurrencyGroup := viper.GetString("orchestrator.submit_concurrency_group")
		cancelInProgress := viper.GetBool("orchestrator.submit_cancel_in_progress")
		agentProvider := viper.GetString("orchestrator.submit_agent_provider")
		agentModel := viper.GetString("orchestrator.submit_agent_model")
		submitAdHocJob(host, submitURL, task, id, priority, delay, timeout, wait, envMap, submitDeps, submitTags, concurrencyGroup, cancelInProgress, agentProvider, agentModel)
		return nil
	}

	if exportFile := viper.GetString("orchestrator.export_jobs"); exportFile != "" {
		host := viper.GetString("orchestrator.host")
		format := viper.GetString("orchestrator.export_format")
		exportJobs(host, exportFile, format)
		return nil
	}
	if importFile := viper.GetString("orchestrator.import_jobs"); importFile != "" {
		host := viper.GetString("orchestrator.host")
		importJobs(host, importFile)
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
	case "cron":
		schedule := viper.GetString("orchestrator.cron_schedule")
		templatePath := viper.GetString("orchestrator.cron_template")
		if schedule == "" || templatePath == "" {
			return fmt.Errorf("cron-schedule and cron-template must be specified in cron poller mode")
		}
		var err error
		poller, err = orchestrator.NewCronPoller(schedule, templatePath)
		if err != nil {
			return fmt.Errorf("Failed to initialize cron poller: %w", err)
		}
		logger.Info("Using cron poller", "schedule", schedule, "template", templatePath)
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
	case "linear":
		token := viper.GetString("orchestrator.linear_token")
		team := viper.GetString("orchestrator.linear_team")
		lnLabel := viper.GetString("orchestrator.linear_label")
		if lnLabel == "" {
			lnLabel = label // Fallback to jira-label
		}

		if token == "" || team == "" {
			return fmt.Errorf("Linear token and team must be specified in linear poller mode")
		}
		poller = orchestrator.NewLinearPoller(token, team, lnLabel)
		logger.Info("Using Linear poller", "team", team, "label", lnLabel)
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
	case "trello":
		key := viper.GetString("orchestrator.trello_key")
		token := viper.GetString("orchestrator.trello_token")
		board := viper.GetString("orchestrator.trello_board")
		list := viper.GetString("orchestrator.trello_list")

		if key == "" || token == "" || (board == "" && list == "") {
			return fmt.Errorf("Trello key, token, and either board or list must be specified in trello poller mode")
		}
		poller = orchestrator.NewTrelloPoller(key, token, board, list)
		logger.Info("Using Trello poller", "board", board, "list", list)
	case "asana":
		token := viper.GetString("orchestrator.asana_token")
		project := viper.GetString("orchestrator.asana_project")

		if token == "" || project == "" {
			return fmt.Errorf("Asana token and project must be specified in asana poller mode")
		}
		poller = orchestrator.NewAsanaPoller(token, project)
		logger.Info("Using Asana poller", "project", project)
	case "notion":
		token := viper.GetString("orchestrator.notion_token")
		dbID := viper.GetString("orchestrator.notion_database_id")
		ntLabel := viper.GetString("orchestrator.notion_label")
		if ntLabel == "" {
			ntLabel = label // Fallback to jira-label
		}

		if token == "" || dbID == "" {
			return fmt.Errorf("Notion token and database ID must be specified in notion poller mode")
		}
		poller = orchestrator.NewNotionPoller(token, dbID, ntLabel)
		logger.Info("Using Notion poller", "database_id", dbID, "label", ntLabel)
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
	case "process", "native":
		spawner = orchestrator.NewProcessSpawner(logger, agentProvider, agentModel, sm, maxIterations, managerFrequency, taskMaxIterations)
	default:
		return fmt.Errorf("Invalid mode. Use 'local', 'k8s', or 'process': %s", mode)
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
	orch.MaxConcurrentJobs = viper.GetInt("orchestrator.max_concurrent_jobs")
	orch.JobTimeout = viper.GetDuration("orchestrator.job_timeout")
	orch.MaxRetries = viper.GetInt("orchestrator.max_retries")
	orch.LogDir = viper.GetString("orchestrator.log_dir")
	orch.RetryDelay = viper.GetDuration("orchestrator.retry_delay")
	orch.RequireApproval = viper.GetBool("orchestrator.require_approval")

	// 5. Notifications
	notifyManager := notify.NewManager(func(msg string, args ...interface{}) {
		logger.Info(fmt.Sprintf(msg, args...))
	})
	notifyManager.Start(ctx)
	orch.SetNotifier(notifyManager)

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

func printAnalytics(host string) {
	url := fmt.Sprintf("%s/analytics", host)
	resp, err := http.Get(url)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to connect to orchestrator at %s: %v\n", host, err)
		exitFunc(1)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(stdout, "Failed to fetch analytics: status %s\n", resp.Status)
		exitFunc(1)
		return
	}

	var analytics orchestrator.Analytics
	if err := json.NewDecoder(resp.Body).Decode(&analytics); err != nil {
		fmt.Fprintf(stdout, "Failed to decode response: %v\n", err)
		exitFunc(1)
		return
	}

	// Styles
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#7D56F4")).
		Padding(0, 1)

	labelStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("86")).
		Width(20)

	valueStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("252"))

	fmt.Fprintln(stdout, titleStyle.Render("Orchestrator Analytics"))
	fmt.Fprintln(stdout, "")

	printField := func(label, value string) {
		fmt.Fprintf(stdout, "%s %s\n", labelStyle.Render(label+":"), valueStyle.Render(value))
	}

	printField("Total Jobs", fmt.Sprintf("%d", analytics.TotalJobs))
	printField("Successful Jobs", fmt.Sprintf("%d", analytics.SuccessfulJobs))
	printField("Failed Jobs", fmt.Sprintf("%d", analytics.FailedJobs))
	printField("Canceled Jobs", fmt.Sprintf("%d", analytics.CanceledJobs))
	printField("Success Rate", fmt.Sprintf("%.2f%%", analytics.SuccessRate))

	// Convert ns to a more readable format, like 1h2m3s
	avgDuration := analytics.AverageDuration.Round(time.Second).String()
	printField("Average Duration", avgDuration)

	if len(analytics.TotalMetrics) > 0 {
		fmt.Fprintln(stdout, "\n"+titleStyle.Render("Total Metrics"))
		for k, v := range analytics.TotalMetrics {
			fmt.Fprintf(stdout, "%s %s\n", labelStyle.Render(k+":"), valueStyle.Render(fmt.Sprintf("%.2f", v)))
		}
	}

	fmt.Fprintln(stdout, "")
}

func printStatus(host string) {
	url := fmt.Sprintf("%s/status", host)
	resp, err := http.Get(url)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to connect to orchestrator at %s: %v\n", host, err)
		exitFunc(1)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(stdout, "Failed to fetch status: status %s\n", resp.Status)
		exitFunc(1)
		return
	}

	var status orchestrator.Status
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		fmt.Fprintf(stdout, "Failed to decode response: %v\n", err)
		exitFunc(1)
		return
	}

	// Styles
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#7D56F4")).
		Padding(0, 1)

	labelStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("86")).
		Width(18)

	valueStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("252"))

	fmt.Fprintln(stdout, titleStyle.Render("Orchestrator Status"))
	fmt.Fprintln(stdout, "")

	printField := func(label, value string) {
		fmt.Fprintf(stdout, "%s %s\n", labelStyle.Render(label+":"), valueStyle.Render(value))
	}

	printField("Uptime", status.Uptime)
	printField("Poll Interval", status.PollInterval)
	if !status.LastPoll.IsZero() {
		printField("Last Poll", status.LastPoll.Format(time.RFC3339))
	} else {
		printField("Last Poll", "N/A")
	}
	printField("Last Poll Items", fmt.Sprintf("%d", status.LastPollItems))
	printField("Active Spawns", fmt.Sprintf("%d", status.ActiveSpawns))
	printField("Pending Jobs", fmt.Sprintf("%d", status.PendingJobs))
	printField("Total Spawns", fmt.Sprintf("%d", status.TotalSpawns))
	printField("Paused", fmt.Sprintf("%t", status.Paused))
	if status.MaxConcurrentJobs > 0 {
		printField("Max Concurrent Jobs", fmt.Sprintf("%d", status.MaxConcurrentJobs))
	} else {
		printField("Max Concurrent Jobs", "Unlimited")
	}
	fmt.Fprintln(stdout, "")
}

func listPendingJobs(host string, format string) {
	u, err := url.Parse(fmt.Sprintf("%s/jobs", host))
	if err != nil {
		fmt.Fprintf(stdout, "Failed to parse host URL: %v\n", err)
		exitFunc(1)
		return
	}

	q := u.Query()
	q.Set("state", "pending")
	u.RawQuery = q.Encode()

	resp, err := http.Get(u.String())
	if err != nil {
		fmt.Fprintf(stdout, "Failed to connect to orchestrator at %s: %v\n", host, err)
		exitFunc(1)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(stdout, "Failed to fetch pending jobs: status %s\n", resp.Status)
		exitFunc(1)
		return
	}

	var jobs []orchestrator.JobInfo
	if err := json.NewDecoder(resp.Body).Decode(&jobs); err != nil {
		fmt.Fprintf(stdout, "Failed to decode response: %v\n", err)
		exitFunc(1)
		return
	}

	if format == "json" {
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(jobs); err != nil {
			fmt.Fprintf(stdout, "Failed to encode jobs to JSON: %v\n", err)
			exitFunc(1)
		}
		return
	}

	if len(jobs) == 0 {
		fmt.Fprintln(stdout, "No pending jobs.")
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

	fmt.Fprintln(stdout, titleStyle.Render(fmt.Sprintf("Pending Jobs (%d)", len(jobs))))
	fmt.Fprintln(stdout, "")

	// Table Header
	fmt.Fprintf(stdout, "%-15s %-40s %-25s %-20s\n",
		headerStyle.Render("ID"),
		headerStyle.Render("Summary"),
		headerStyle.Render("Status"),
		headerStyle.Render("Duration"),
	)

	for _, job := range jobs {
		duration := time.Since(job.StartTime).Round(time.Second).String()
		statusDisplay := job.Status
		if job.Progress != nil {
			statusDisplay = fmt.Sprintf("%s (%d%%)", job.Status, *job.Progress)
		}
		if job.StatusMessage != nil {
			statusDisplay = fmt.Sprintf("%s - %s", statusDisplay, *job.StatusMessage)
		}
		statusDisplay = limitString(statusDisplay, 25)

		fmt.Fprintf(stdout, "%-15s %-40s %-25s %-20s\n",
			rowStyle.Render(job.ID),
			rowStyle.Render(limitString(job.Summary, 38)),
			rowStyle.Render(statusDisplay),
			rowStyle.Render(duration),
		)
	}
}

func listJobs(host string, history bool, status, tag, match, format string) {
	u, err := url.Parse(fmt.Sprintf("%s/jobs", host))
	if err != nil {
		fmt.Fprintf(stdout, "Failed to parse host URL: %v\n", err)
		exitFunc(1)
		return
	}

	q := u.Query()
	if history {
		q.Set("state", "all")
	}
	if status != "" {
		q.Set("status", status)
	}
	if tag != "" {
		q.Set("tag", tag)
	}
	if match != "" {
		q.Set("match", match)
	}
	u.RawQuery = q.Encode()

	resp, err := http.Get(u.String())
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

	if format == "json" {
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(jobs); err != nil {
			fmt.Fprintf(stdout, "Failed to encode jobs to JSON: %v\n", err)
			exitFunc(1)
		}
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
	fmt.Fprintf(stdout, "%-15s %-40s %-25s %-20s\n",
		headerStyle.Render("ID"),
		headerStyle.Render("Summary"),
		headerStyle.Render("Status"),
		headerStyle.Render("Duration"),
	)

	for _, job := range jobs {
		duration := time.Since(job.StartTime).Round(time.Second).String()
		statusDisplay := job.Status
		if job.Progress != nil {
			statusDisplay = fmt.Sprintf("%s (%d%%)", job.Status, *job.Progress)
		}
		if job.StatusMessage != nil {
			statusDisplay = fmt.Sprintf("%s - %s", statusDisplay, *job.StatusMessage)
		}
		statusDisplay = limitString(statusDisplay, 25)

		fmt.Fprintf(stdout, "%-15s %-40s %-25s %-20s\n",
			rowStyle.Render(job.ID),
			rowStyle.Render(limitString(job.Summary, 38)),
			rowStyle.Render(statusDisplay),
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

	// Tags
	if len(job.WorkItem.Tags) > 0 {
		fmt.Fprintln(stdout, labelStyle.Render("Tags:"))
		fmt.Fprintln(stdout, lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("  "+strings.Join(job.WorkItem.Tags, ", ")))
		fmt.Fprintln(stdout, "")
	}

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

	// Metrics
	if len(job.Metrics) > 0 {
		fmt.Fprintln(stdout, "\n"+labelStyle.Render("Metrics:"))
		for k, v := range job.Metrics {
			fmt.Fprintf(stdout, "  %s=%.2f\n", k, v)
		}
	}
}

func purgeJob(host, jobID string) {
	req, err := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/history/%s", host, jobID), nil)
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
		fmt.Fprintf(stdout, "Failed to purge job: %s\n", strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	fmt.Fprintf(stdout, "Job %s purged successfully.\n", jobID)
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

func drainOrchestrator(host string) {
	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/drain", host), nil)
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
		fmt.Fprintf(stdout, "Failed to set orchestrator to drain mode: %s\n", strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	fmt.Fprintln(stdout, "Orchestrator is now draining.")
}

func undrainOrchestrator(host string) {
	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/undrain", host), nil)
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
		fmt.Fprintf(stdout, "Failed to remove orchestrator from drain mode: %s\n", strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	fmt.Fprintln(stdout, "Orchestrator is no longer draining.")
}

func forcePoll(host string) {
	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/poll", host), nil)
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
		fmt.Fprintf(stdout, "Failed to force poll orchestrator: %s\n", strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	fmt.Fprintln(stdout, "Orchestrator poll triggered.")
}

func scaleConcurrency(host string, max int) {
	reqBody := fmt.Sprintf(`{"max_concurrent_jobs": %d}`, max)
	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/scale", host), strings.NewReader(reqBody))
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
		fmt.Fprintf(stdout, "Failed to scale orchestrator: %s\n", strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	fmt.Fprintf(stdout, "Orchestrator concurrency limit scaled to %d.\n", max)
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

func retryFailedJobs(host, match string, tag string) {
	u, err := url.Parse(fmt.Sprintf("%s/jobs/retry-failed", host))
	if err != nil {
		fmt.Fprintf(stdout, "Failed to parse URL: %v\n", err)
		exitFunc(1)
		return
	}

	q := u.Query()
	if match != "" {
		q.Set("match", match)
	}
	if tag != "" {
		q.Set("tag", tag)
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequest(http.MethodPost, u.String(), nil)
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

func approveJob(host, jobID string) {
	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/jobs/%s/approve", host, jobID), nil)
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
		fmt.Fprintf(stdout, "Failed to approve job: %s\n", strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	fmt.Fprintf(stdout, "Job %s approved successfully.\n", jobID)
}

func holdJob(host, jobID string) {
	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/jobs/%s/hold", host, jobID), nil)
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
		fmt.Fprintf(stdout, "Failed to hold job: %s\n", strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	fmt.Fprintf(stdout, "Job %s held successfully.\n", jobID)
}

func unholdJob(host, jobID string) {
	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/jobs/%s/unhold", host, jobID), nil)
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
		fmt.Fprintf(stdout, "Failed to unhold job: %s\n", strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	fmt.Fprintf(stdout, "Job %s unheld successfully.\n", jobID)
}
