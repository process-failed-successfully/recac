package main

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
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
	"recac/internal/utils"

	"github.com/charmbracelet/lipgloss"
	"github.com/joho/godotenv"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
)

func expandAliases(args []string) []string {
	aliases := viper.GetStringMapString("orchestrator.aliases")
	if len(aliases) == 0 || len(args) <= 1 {
		return args
	}

	// We look for the first argument that doesn't start with a dash (-)
	// and isn't the value of a previous flag (like --config value)

	// A quick list of string flags that we know might precede an alias
	// Ideally we'd use pflag to parse, but we are expanding before pflag.Parse().
	// For robust alias expansion, we just skip any argument starting with `-`
	// and if a previous argument was a known flag that takes a value, we skip the current one too.
	skipNext := false

	for i := 1; i < len(args); i++ {
		arg := args[i]

		if skipNext {
			skipNext = false
			continue
		}

		if strings.HasPrefix(arg, "-") {
			// Some common flags that take arguments without '='
			if arg == "--config" || arg == "-c" || arg == "--list-jobs-status" || arg == "--list-jobs-tag" || arg == "--list-jobs-match" || arg == "--list-jobs-format" {
				skipNext = true
			}
			continue
		}

		// This is the first positional argument. Expand it if it's an alias.
		if replacement, ok := aliases[arg]; ok {
			parts := strings.Fields(replacement)

			var newArgs []string
			newArgs = append(newArgs, args[:i]...)
			newArgs = append(newArgs, parts...)
			newArgs = append(newArgs, args[i+1:]...)

			return newArgs
		}

		// If we found a positional argument but it wasn't an alias, stop looking
		// (aliases only apply to the main subcommand)
		break
	}

	return args
}

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
	pflag.Bool("list-tags", false, "List all unique tags across all jobs and their counts")
	pflag.Bool("list-groups", false, "List all concurrency groups and their statuses")
	pflag.String("list-dependents", "", "List downstream dependent jobs for a specific job ID")
	pflag.String("list-blockers", "", "List upstream blocking jobs for a specific job ID")
	pflag.String("list-jobs-status", "", "Filter jobs by status (e.g., Running, Failed, Completed)")
	pflag.String("list-jobs-tag", "", "Filter jobs by a specific tag")
	pflag.String("list-jobs-match", "", "Filter jobs by a regex matching the summary or error")
	pflag.String("list-jobs-priority", "", "Filter jobs by a specific priority")
	pflag.String("search-jobs", "", "Search pending, active, and completed jobs by regex query")
	pflag.String("list-jobs-format", "table", "Output format for list-jobs and list-pending (table, json, csv)")
	pflag.String("format", "text", "Output format for status and analytics (text, json)")
	pflag.Bool("summary", false, "Get a summary of job counts by status")
	pflag.Bool("watch", false, "Continuously watch the output of list-jobs or list-pending")
	pflag.Duration("watch-interval", 2*time.Second, "Refresh interval for watch mode (e.g. 2s, 1m)")
	pflag.Bool("status", false, "Get the current status of the orchestrator")
	pflag.Bool("tail-active", false, "Tail logs from all currently active jobs simultaneously")
	pflag.String("tail-tag", "", "Filter tailed active jobs by a specific tag")
	pflag.String("tail-match", "", "Filter tailed active jobs matching a regex")
	pflag.String("tail-group", "", "Filter tailed active jobs by a specific concurrency group")
	pflag.String("tail-job", "", "Tail logs from a specific currently active job continuously")
	pflag.Bool("analytics", false, "Show orchestrator analytics")
	pflag.Bool("critical-path", false, "Analyze and display the critical path of job execution")
	pflag.Bool("tree", false, "Display the dependency tree of jobs")
	pflag.String("tree-job", "", "Display the dependency tree for a specific job")
	pflag.Bool("timeline", false, "Display an execution timeline (Gantt chart) of jobs")
	pflag.Int("timeline-limit", 20, "Limit the number of jobs displayed in the timeline")
	pflag.Bool("monitor", false, "Launch the TUI dashboard to monitor the orchestrator")
	pflag.Bool("stream-events", false, "Stream real-time orchestrator events (SSE) to the console")
	pflag.String("logs", "", "Get logs for a specific job ID from a running orchestrator instance")
	pflag.String("edit-job", "", "Edit a pending job interactively using $EDITOR")
	pflag.Bool("build-job", false, "Interactively build and submit a new ad-hoc job")
	pflag.String("inspect-job", "", "Inspect a specific job by ID")
	pflag.String("inspect-dataflow", "", "Inspect how upstream job outputs are injected as environment variables into a specific job by ID")
	pflag.String("explain-job", "", "Use AI to analyze and explain why a job failed by ID")
	pflag.String("explain-tag", "", "Use AI to analyze and explain why jobs with the specified tag failed")
	pflag.String("explain-match", "", "Use AI to analyze and explain why jobs matching the given regex failed")
	pflag.Bool("analyze-failures", false, "Analyze and group all failed jobs by their summary error signature")
	pflag.Bool("analyze-durations", false, "Analyze and display a statistical breakdown of job execution times")
	pflag.Int("analyze-durations-limit", 10, "Limit the number of slowest jobs displayed in duration analysis")
	pflag.Bool("analyze-reliability", false, "Analyze and display pipeline reliability stats, identifying flaky and failing jobs")
	pflag.Int("analyze-reliability-limit", 10, "Limit the number of top flaky and failing jobs displayed in reliability analysis")
	pflag.Bool("analyze-costs", false, "Analyze and display a statistical breakdown of job costs")
	pflag.Int("analyze-costs-limit", 10, "Limit the number of top expensive jobs displayed in cost analysis")
	pflag.Bool("analyze-anomalies", false, "Analyze and display jobs whose duration or cost exceeds 2 standard deviations from the model's mean")
	pflag.Int("analyze-anomalies-limit", 10, "Limit the number of anomalies displayed in anomaly analysis")
	pflag.Bool("analyze-agents", false, "Analyze and display a statistical breakdown of agent model performance")
	pflag.Bool("analyze-tags", false, "Analyze and display a statistical breakdown of job tags")
	pflag.Int("analyze-tags-limit", 10, "Limit the number of tags displayed in tag analysis")
	pflag.Int("analyze-agents-limit", 10, "Limit the number of agents displayed in agent analysis")
	pflag.String("heal-job", "", "Retrieve failed job, construct a new one embedding failure context, append auto-heal tag, and resubmit")
	pflag.String("heal-match", "", "Heal all failed jobs matching the given regex")
	pflag.String("heal-tag", "", "Heal all failed jobs with the specified tag")
	pflag.String("compare-jobs", "", "Compare two jobs by ID (comma-separated, e.g. job1,job2)")
	pflag.String("cancel-job", "", "Cancel a running job by ID")
	pflag.Bool("cancel-all", false, "Cancel all currently running jobs")
	pflag.String("cancel-tag", "", "Cancel all active and pending jobs with the specified tag")
	pflag.String("cancel-status", "", "Cancel all active and pending jobs with the specified status")
	pflag.String("cancel-match", "", "Cancel all active and pending jobs matching the given regex")
	pflag.String("cancel-group", "", "Cancel all active and pending jobs with the specified concurrency group")
	pflag.String("cancel-older-than", "", "Cancel all active and pending jobs older than the specified duration (e.g., 24h, 168h)")
	pflag.String("pause-group", "", "Pause a specific concurrency group")
	pflag.String("resume-group", "", "Resume a specific concurrency group")
	pflag.String("purge-job", "", "Purge a specific job from history")
	pflag.String("purge-tag", "", "Purge all completed/failed jobs with the specified tag from history")
	pflag.String("purge-status", "", "Purge all history jobs with the specified status")
	pflag.String("purge-match", "", "Purge all history jobs matching the given regex")
	pflag.String("purge-group", "", "Purge all completed/failed jobs matching the given concurrency group from history")
	pflag.String("purge-older-than", "", "Purge all history jobs older than the specified duration (e.g., 24h, 168h)")
	pflag.Bool("purge-failed", false, "Purge all failed jobs from history")
	pflag.Bool("clear-history", false, "Clear all completed and failed jobs from history")
	pflag.Bool("clear-pending", false, "Clear all jobs waiting in the pending queue")
	pflag.Bool("clean-all", false, "Cancel all active jobs, clear pending queue, and clear history")
	pflag.String("delete-pending-job", "", "Delete a specific job from the pending queue")
	pflag.String("delete-pending-tag", "", "Delete all pending jobs with the specified tag")
	pflag.String("delete-pending-match", "", "Delete all pending jobs matching the given regex")
	pflag.String("delete-pending-group", "", "Delete all pending jobs with the specified concurrency group")
	pflag.String("delete-pending-older-than", "", "Delete all pending jobs older than the specified duration (e.g. 24h, 30m)")
	pflag.String("retry-job", "", "Retry a completed job by ID")
	pflag.Bool("downstream", false, "Retry the job and all its downstream dependencies (with --retry-job)")
	pflag.String("retry-edit-job", "", "Interactively edit and retry a failed job by ID")
	pflag.String("clone-job", "", "Clone an existing job by ID")
	pflag.String("clone-match", "", "Clone all jobs matching the given regex")
	pflag.String("clone-tag", "", "Clone all jobs with the specified tag")
	pflag.String("clone-group", "", "Clone all jobs with the specified concurrency group")
	pflag.Bool("clone-remap-deps", false, "Remap dependencies between cloned jobs (used with clone-match, clone-tag, or clone-group)")
	pflag.Bool("retry-failed", false, "Retry all failed jobs from history")
	pflag.String("retry-match", "", "Optional regex to match against error messages when retrying failed jobs")
	pflag.String("retry-tag", "", "Retry all failed jobs from history with the specified tag")
	pflag.String("retry-group", "", "Retry all failed jobs from history within the specified concurrency group")
	pflag.Bool("require-approval", false, "Require human approval before starting any job")
	pflag.String("approve-job", "", "Approve a job that is pending approval")
	pflag.String("approve-tag", "", "Approve all pending jobs with the specified tag")
	pflag.String("approve-match", "", "Approve all pending jobs matching the given regex")
	pflag.String("approve-group", "", "Approve all pending jobs within the specified concurrency group")
	pflag.String("approve-older-than", "", "Approve pending jobs older than the specified duration (e.g. 24h, 30m)")
	pflag.Bool("approve-interactive", false, "Interactively approve, skip, or cancel jobs that are pending approval")
	pflag.String("hold-job", "", "Hold a pending job to prevent it from running")
	pflag.String("unhold-job", "", "Unhold a pending job to allow it to run")
	pflag.String("hold-tag", "", "Hold all pending jobs with the specified tag")
	pflag.String("hold-match", "", "Hold all pending jobs matching the given regex")
	pflag.String("hold-group", "", "Hold all pending jobs within the specified concurrency group")
	pflag.String("unhold-tag", "", "Unhold all pending jobs with the specified tag")
	pflag.String("unhold-match", "", "Unhold all pending jobs matching the given regex")
	pflag.String("unhold-group", "", "Unhold all pending jobs within the specified concurrency group")
	pflag.String("rename-job", "", "Rename a pending job by ID")
	pflag.String("new-job-id", "", "New ID for the job (requires --rename-job)")
	pflag.String("skip-job", "", "Skip a specific pending job by ID")
	pflag.String("skip-tag", "", "Skip all pending jobs with the specified tag")
	pflag.String("skip-match", "", "Skip all pending jobs matching the given regex")
	pflag.String("skip-group", "", "Skip all pending jobs within the specified concurrency group")
	pflag.String("skip-older-than", "", "Skip pending jobs older than the specified duration (e.g. 24h, 30m)")
	pflag.String("force-complete-job", "", "Force mark an active, pending, or failed job as completed by ID")
	pflag.String("force-complete-tag", "", "Force mark jobs with the specified tag as completed")
	pflag.String("force-complete-match", "", "Force mark jobs matching the given regex as completed")
	pflag.String("fail-job", "", "Force mark an active or pending job as failed by ID")
	pflag.String("fail-tag", "", "Force mark jobs with the specified tag as failed")
	pflag.String("fail-match", "", "Force mark jobs matching the given regex as failed")
	pflag.String("fail-group", "", "Force mark jobs within the specified concurrency group as failed")
	pflag.Bool("diagnose", false, "Diagnose pending jobs for unresolvable dependencies and deadlocks")
	pflag.Bool("simulate", false, "Simulate orchestrator execution to estimate time to completion")
	pflag.String("simulate-pipeline", "", "Simulate execution of a pipeline YAML file to estimate time to completion")
	pflag.String("simulate-pipeline-out", "", "Output file for the pipeline simulation JSON report (use '-' or leave empty for stdout)")
	pflag.Bool("pause", false, "Pause the orchestrator polling loop")
	pflag.Bool("resume", false, "Resume the orchestrator polling loop")
	pflag.Bool("drain", false, "Set the orchestrator to drain mode")
	pflag.Bool("undrain", false, "Remove the orchestrator from drain mode")
	pflag.Bool("force-poll", false, "Force an immediate poll cycle")
	pflag.Int("scale", -1, "Dynamically scale the maximum concurrent jobs limit")
	pflag.String("update-interval", "", "Update the orchestrator polling interval dynamically")
	pflag.String("update-priority", "", "Update the priority of a specific pending job")
	pflag.String("update-priority-tag", "", "Update the priority of all pending jobs with the specified tag")
	pflag.String("update-priority-match", "", "Update the priority of all pending jobs matching the given regex")
	pflag.String("update-priority-group", "", "Update the priority of all pending jobs with the specified concurrency group")
	pflag.Int("priority-val", 0, "The new priority value to assign (requires --update-priority)")
	pflag.String("promote-job", "", "Promote a specific pending job to run next by bumping its priority to max")
	pflag.String("promote-tag", "", "Promote all pending jobs with the specified tag")
	pflag.String("promote-match", "", "Promote all pending jobs matching the given regex")
	pflag.String("promote-group", "", "Promote all pending jobs with the specified concurrency group")
	pflag.String("demote-job", "", "Demote a specific pending job to run last by dropping its priority to min")
	pflag.String("demote-tag", "", "Demote all pending jobs with the specified tag")
	pflag.String("demote-match", "", "Demote all pending jobs matching the given regex")
	pflag.String("demote-group", "", "Demote all pending jobs with the specified concurrency group")
	pflag.String("update-timeout", "", "Update the timeout of a specific pending job")
	pflag.String("update-timeout-tag", "", "Update the timeout of all pending jobs with the specified tag")
	pflag.String("update-timeout-match", "", "Update the timeout of all pending jobs matching the given regex")
	pflag.String("timeout-val", "", "The new timeout value to assign (e.g., 30m) (requires --update-timeout, --update-timeout-tag, or --update-timeout-match)")
	pflag.String("update-max-retries-job", "", "Update the maximum retries of a specific pending job")
	pflag.String("update-max-retries-tag", "", "Update the maximum retries of all pending jobs with the specified tag")
	pflag.String("update-max-retries-match", "", "Update the maximum retries of all pending jobs matching the given regex")
	pflag.Int("max-retries-val", -1, "The new maximum retries value to assign (requires --update-max-retries-job, --update-max-retries-tag, or --update-max-retries-match)")
	pflag.String("update-agent-job", "", "Update the agent provider and model of a specific pending job")
	pflag.String("update-agent-tag", "", "Update the agent provider and model of all pending jobs with the specified tag")
	pflag.String("update-agent-match", "", "Update the agent provider and model of all pending jobs matching the given regex")
	pflag.String("agent-provider-val", "", "The new agent provider to assign (requires --update-agent-job)")
	pflag.String("agent-model-val", "", "The new agent model to assign (requires --update-agent-job)")
	pflag.String("set-progress-job", "", "Set progress for a specific job")
	pflag.Int("progress-val", -1, "The progress value to set (0-100) (requires --set-progress-job)")
	pflag.String("progress-msg", "", "Optional status message to set along with progress")
	pflag.String("update-deps-job", "", "Update the dependencies of a specific pending job")
	pflag.String("update-deps-tag", "", "Update the dependencies of all pending jobs with the specified tag")
	pflag.String("update-deps-match", "", "Update the dependencies of all pending jobs matching the given regex")
	pflag.StringSlice("set-deps", []string{}, "Comma-separated list of new dependencies (requires --update-deps-job, --update-deps-tag, or --update-deps-match)")
	pflag.String("update-env-job", "", "Update the environment variables of a specific pending job")
	pflag.String("update-env-tag", "", "Update the environment variables of all pending jobs with the specified tag")
	pflag.String("update-env-match", "", "Update the environment variables of all pending jobs matching the given regex")
	pflag.StringSlice("set-env", []string{}, "Comma-separated list of new environment variables (requires --update-env-job, --update-env-tag, or --update-env-match)")
	pflag.String("update-tags-job", "", "Update the tags of a specific pending job")
	pflag.String("update-tags-tag", "", "Update the tags of all pending jobs with the specified tag")
	pflag.String("update-tags-match", "", "Update the tags of all pending jobs matching the given regex")

	pflag.String("add-tag-job", "", "Add tags to a specific pending job")
	pflag.String("add-tag-tag", "", "Add tags to all pending jobs with the specified tag")
	pflag.String("add-tag-match", "", "Add tags to all pending jobs matching the given regex")
	pflag.String("remove-tag-job", "", "Remove tags from a specific pending job")
	pflag.String("remove-tag-tag", "", "Remove tags from all pending jobs with the specified tag")
	pflag.String("remove-tag-match", "", "Remove tags from all pending jobs matching the given regex")

	pflag.StringSlice("set-tags", []string{}, "Comma-separated list of new tags (requires --update-tags-job, --update-tags-tag, or --update-tags-match)")
	pflag.String("wait-job", "", "Wait for a specific job to complete and stream its logs")
	pflag.String("wait-jobs", "", "Wait for multiple comma-separated job IDs to complete")
	pflag.String("wait-tag", "", "Wait for all jobs with a specific tag to complete")
	pflag.String("wait-match", "", "Wait for all jobs matching a regex to complete")
	pflag.String("wait-group", "", "Wait for all jobs with a specific concurrency group to complete")
	pflag.Bool("wait-idle", false, "Wait for the orchestrator to become completely idle (0 active and 0 pending jobs)")
	pflag.String("set-output-job", "", "Set output key-value pair for a job")
	pflag.String("set-output-key", "", "Output key (requires --set-output-job)")
	pflag.String("set-output-val", "", "Output value (requires --set-output-job)")
	pflag.String("get-output-job", "", "Get specific output value for a job by ID")
	pflag.String("get-output-key", "", "Output key to retrieve (requires --get-output-job)")
	pflag.String("get-metrics-job", "", "Get specific metrics value for a job by ID")
	pflag.String("get-metrics-key", "", "Metrics key to retrieve (requires --get-metrics-job)")
	pflag.String("add-metrics-job", "", "Add metrics to a specific job")
	pflag.String("metrics-key", "", "The metrics key to add (requires --add-metrics-job)")
	pflag.Float64("metrics-val", 0, "The metrics value to add (requires --add-metrics-job)")
	pflag.String("archive-job", "", "Download a compressed archive containing job details and logs")
	pflag.String("archive-out", "", "Output path for the archive file (default is {jobID}.tar.gz) (used with --archive-job)")
	pflag.String("archive-tag", "", "Download a compressed archive containing job details and logs for all jobs with a specific tag")
	pflag.String("archive-match", "", "Download a compressed archive containing job details and logs for all jobs matching a regex")
	pflag.Bool("archive-failed", false, "Download a compressed archive containing job details and logs for all failed jobs")
	pflag.String("archive-status", "", "Download a compressed archive containing job details and logs for all jobs with a specific status")
	pflag.String("archive-group", "", "Download a compressed archive containing job details and logs for all jobs within a specific concurrency group")
	pflag.String("archive-older-than", "", "Download a compressed archive containing job details and logs for all jobs older than the specified duration (e.g. 24h, 30m)")
	pflag.String("submit", "", "Submit a job from a JSON file path")
	pflag.String("submit-batch", "", "Submit multiple jobs from a JSON file path")
	pflag.String("submit-matrix", "", "Submit a matrix job from a JSON file path")
	pflag.String("submit-pipeline", "", "Submit a pipeline job from a YAML file path")
	pflag.String("validate-pipeline", "", "Validate a pipeline job from a YAML file path")
	pflag.String("submit-pipeline-target", "", "Submit only the specified target job and its dependencies from a pipeline YAML file")
	pflag.StringSlice("pipeline-var", []string{}, "Variables to substitute in the pipeline YAML (e.g. --pipeline-var KEY=VALUE)")
	pflag.StringArray("submit-matrix-inline", []string{}, "Submit a matrix job inline (e.g. --submit-matrix-inline OS=linux,windows --submit-matrix-inline GO=1.20,1.21)")
	pflag.String("pipeline-var-file", "", "Path to a file containing variables to substitute in the pipeline YAML (.json, .yaml, .yml, or .env)")
	pflag.Bool("submit-pipeline-interactive", false, "Interactively edit pipeline variables before submitting")
	pflag.Bool("dry-run-pipeline", false, "Preview the jobs generated by a pipeline YAML file without submitting")
	pflag.String("lint-pipeline", "", "Validate a pipeline YAML file without submitting")
	pflag.String("import-pipeline", "", "Import a pipeline YAML file and hold all generated jobs")
	pflag.String("explain-pipeline", "", "Explain a pipeline YAML file (visualize execution structure) without submitting")
	pflag.String("export-pipeline-graph", "", "Export a pipeline YAML file to a visual graph format (use '-' for stdout) without submitting")
	pflag.String("export-pipeline-graph-format", "mermaid", "Format for exported pipeline graph ('mermaid', 'dot', or 'plantuml')")
	pflag.String("export-pipeline-graph-out", "", "Output file for the pipeline graph (use '-' or leave empty for stdout)")
	pflag.String("list-templates", "", "List all templates defined in a pipeline YAML file")
	pflag.String("list-pipeline-vars", "", "List all required and declared variables in a pipeline YAML file")
	pflag.String("inspect-pipeline", "", "Inspect a pipeline YAML file and display its resolved jobs visually")
	pflag.String("compare-pipelines", "", "Compare two pipeline YAML files (comma-separated, e.g., p1.yaml,p2.yaml)")
	pflag.String("apply-pipeline", "", "Apply a pipeline YAML file declaratively (creates missing, updates pending, skips active jobs)")
	pflag.String("watch-pipeline", "", "Watch a pipeline YAML file for changes and automatically apply it")
	pflag.Duration("watch-pipeline-interval", 2*time.Second, "Refresh interval for watch-pipeline mode (e.g. 2s, 1m)")
	pflag.Bool("apply-pipeline-dry-run", false, "Preview changes that apply-pipeline would make without applying them")
	pflag.String("apply-pipeline-run-id", "stable", "Run ID to append to job IDs (use 'stable' to omit suffix entirely, ensuring stable IDs for declarative updates)")
	pflag.String("search-logs", "", "Search logs of all active and completed jobs for a regex pattern")
	pflag.String("search-tag", "", "Optional tag filter when searching logs")
	pflag.String("search-status", "", "Optional status filter when searching logs")
	pflag.Int("search-context", 0, "Number of lines of context to include before and after the match")
	pflag.String("submit-url", "", "Repo URL for ad-hoc job submission")
	pflag.String("submit-task", "", "Task description for ad-hoc job submission")
	pflag.String("submit-id", "", "Optional ID for ad-hoc job submission")
	pflag.Int("submit-priority", 0, "Priority for ad-hoc job submission (higher is more important)")
	pflag.Duration("submit-delay", 0, "Delay before starting the ad-hoc job (e.g., 1m, 1h)")
	pflag.StringSlice("env", []string{}, "Environment variables to pass to the ad-hoc job (e.g., --env KEY=VALUE)")
	pflag.StringSlice("submit-deps", []string{}, "Comma-separated list of job IDs this job depends on")
	pflag.StringSlice("submit-tags", []string{}, "Comma-separated list of tags for the ad-hoc job")
	pflag.Duration("submit-timeout", 0, "Optional custom timeout for the ad-hoc job (e.g. 30m)")
	pflag.Duration("submit-dependency-timeout", 0, "Optional dependency wait timeout for the ad-hoc job (e.g. 1h)")
	pflag.Int("submit-max-retries", -1, "Maximum retries for the ad-hoc job (-1 to use global default)")
	pflag.Bool("submit-require-approval", false, "Require approval before executing the ad-hoc job")
	pflag.Duration("submit-retry-delay", 0, "Optional custom retry delay for the ad-hoc job (e.g. 5s)")
	pflag.Float64("submit-retry-backoff", 1.0, "Optional retry backoff multiplier for the ad-hoc job")
	pflag.String("submit-concurrency-group", "", "Concurrency group for the ad-hoc job")
	pflag.Bool("submit-cancel-in-progress", false, "Cancel running jobs in the same concurrency group")
	pflag.String("submit-agent-provider", "", "Agent provider to use for the ad-hoc job")
	pflag.String("submit-agent-model", "", "Agent model to use for the ad-hoc job")
	pflag.String("submit-run-condition", "", "Run condition for the ad-hoc job (always, on_failure, on_success)")
	pflag.String("submit-webhook-url", "", "Webhook URL to call when job completes")
	pflag.Bool("submit-auto-heal", false, "Enable auto-healing: automatically append failure logs to description on retries")
	pflag.Bool("wait", false, "Wait for job completion and stream logs (for submit/submit-url)")
	pflag.String("host", "http://localhost:2112", "Orchestrator host URL (for list-jobs, logs, cancel-job, and submit)")

	pflag.String("export-job", "", "Export a single job's WorkItem configuration to JSON format")
	pflag.String("export-job-out", "", "Output file for the exported job (use '-' or leave empty for stdout)")
	pflag.String("export-jobs", "", "Export all jobs to a file (use '-' for stdout)")
	pflag.String("import-jobs", "", "Import jobs from an exported JSON file")
	pflag.String("export-format", "json", "Format for exported jobs ('json', 'csv', or 'junit')")
	pflag.String("export-pipeline", "", "Export active and pending jobs as a pipeline YAML (use '-' for stdout)")
	pflag.String("export-graph", "", "Export the job dependency graph (use '-' for stdout)")
	pflag.String("export-graph-format", "mermaid", "Format for exported graph ('mermaid', 'dot', or 'plantuml')")
	pflag.String("export-metrics", "", "Export metrics for jobs to a CSV file (use '-' for stdout)")
	pflag.String("export-metrics-state", "all", "State of jobs to export metrics for ('all', 'active', 'completed', 'failed')")
	pflag.String("export-junit", "", "Export jobs as a JUnit XML report to a file (use '-' for stdout)")
	pflag.String("export-trace", "", "Export jobs as Chrome Trace Event format to a JSON file (use '-' for stdout)")
	pflag.String("export-trace-state", "all", "State of jobs to export trace for ('all', 'active', 'completed', 'failed')")
	pflag.String("export-timeline", "", "Export jobs as a Mermaid Gantt chart to a text file (use '-' for stdout)")
	pflag.String("export-timeline-state", "all", "State of jobs to export timeline for ('all', 'active', 'completed', 'failed')")
	pflag.String("export-costs", "", "Export cost analysis to a file (use '-' for stdout)")
	pflag.String("export-costs-format", "json", "Format for exported costs ('json' or 'csv')")
	pflag.String("export-agents", "", "Export agent analysis to a file (use '-' for stdout)")
	pflag.String("export-agents-format", "json", "Format for exported agents ('json' or 'csv')")
	pflag.String("export-durations", "", "Export job durations analysis to a file (use '-' for stdout)")
	pflag.String("export-durations-format", "json", "Format for exported durations ('json' or 'csv')")
	pflag.String("export-reliability", "", "Export job reliability analysis to a file (use '-' for stdout)")
	pflag.String("export-reliability-format", "json", "Format for exported reliability ('json' or 'csv')")
	pflag.String("export-failures", "", "Export failures analysis to a file (use '-' for stdout)")
	pflag.String("export-failures-format", "json", "Format for exported failures ('json' or 'csv')")
	pflag.String("export-anomalies", "", "Export anomalies analysis to a file (use '-' for stdout)")
	pflag.String("export-anomalies-format", "json", "Format for exported anomalies ('json' or 'csv')")
	pflag.String("export-tags", "", "Export tag analysis to a file (use '-' for stdout)")
	pflag.String("export-tags-format", "json", "Format for exported tags ('json' or 'csv')")

	pflag.String("upload-artifact", "", "Path to the local file to upload as an artifact (requires --job-id)")
	pflag.String("download-artifact", "", "Filename of the artifact to download (requires --job-id)")
	pflag.String("artifact-out", "", "Output path for the downloaded artifact (used with --download-artifact)")
	pflag.Bool("list-artifacts", false, "List all artifacts for a specific job (requires --job-id)")
	pflag.String("delete-artifact", "", "Filename of the artifact to delete (requires --job-id)")
	pflag.String("job-id", "", "Job ID to operate on (used with artifact commands)")

	pflag.String("generate-pipeline", "", "Generate a pipeline YAML using AI based on the provided prompt")
	pflag.String("generate-pipeline-out", "", "Output file for the generated pipeline YAML (use '-' or leave empty for stdout)")

	pflag.String("generate-changelog", "", "Generate a markdown changelog from completed jobs (use '-' or leave empty for stdout). Optionally provide the output file path.")
	pflag.String("changelog-tag", "", "Optional tag filter for the changelog generation")
	pflag.String("changelog-match", "", "Optional regex filter for the changelog generation")

	pflag.String("generate-postmortem", "", "Generate a markdown postmortem report from failed jobs (use '-' or leave empty for stdout). Optionally provide the output file path.")
	pflag.String("postmortem-tag", "", "Optional tag filter for the postmortem generation")
	pflag.String("postmortem-match", "", "Optional regex filter for the postmortem generation")

	pflag.String("optimize-pipeline", "", "Analyze a pipeline YAML file and use AI to suggest optimizations")
	pflag.String("optimize-pipeline-out", "", "Output file for the optimized pipeline YAML (use '-' or leave empty for stdout)")

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
	pflag.Float64("max-budget", 0, "Maximum total cost budget before the orchestrator automatically pauses")
	pflag.String("log-dir", "", "Directory to store persistent compressed job logs")
	pflag.String("artifacts-dir", "", "Directory to store job artifacts")
	pflag.Duration("retry-delay", 5*time.Second, "Delay between automatic retries")
	pflag.Int("circuit-breaker-max", 5, "Number of consecutive spawn failures before circuit breaker trips (0 to disable)")

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
	pflag.String("linear-webhook-secret", "", "Linear Webhook Secret for validating incoming POST events")

	pflag.String("gitlab-token", "", "GitLab API Token (for 'gitlab' poller)")
	pflag.String("gitlab-project", "", "GitLab Project ID or URL-encoded path (for 'gitlab' poller)")
	pflag.String("gitlab-label", "", "GitLab Label to poll for (defaults to jira-label if not set)")
	pflag.String("gitlab-url", "", "GitLab URL (defaults to https://gitlab.com)")
	pflag.String("gitlab-webhook-secret", "", "GitLab Webhook Secret for validating incoming POST events")
	pflag.String("jira-webhook-secret", "", "Jira Webhook Secret for validating incoming POST events")

	pflag.String("trello-key", "", "Trello API Key (for 'trello' poller)")
	pflag.String("trello-token", "", "Trello API Token (for 'trello' poller)")
	pflag.String("trello-board", "", "Trello Board ID (for 'trello' poller)")
	pflag.String("trello-list", "", "Trello List ID to poll for (for 'trello' poller)")
	pflag.String("trello-webhook-secret", "", "Trello Webhook Secret for validating incoming POST events")

	pflag.String("asana-token", "", "Asana API Token (for 'asana' poller)")
	pflag.String("asana-project", "", "Asana Project ID (for 'asana' poller)")

	pflag.String("notion-token", "", "Notion API Token (for 'notion' poller)")
	pflag.String("notion-database-id", "", "Notion Database ID (for 'notion' poller)")
	pflag.String("notion-label", "", "Notion Label/Tag to poll for (defaults to jira-label if not set)")

	pflag.StringSlice("allowed-pollers", []string{}, "Comma-separated list of allowed poller types")

	pflag.Bool("webhook-enabled", false, "Enable generic webhook notifications")
	pflag.String("webhook-url", "", "URL for generic webhook notifications")
	pflag.String("webhook-secret", "", "Secret for generic webhook HMAC signature")
	pflag.Bool("generic-webhook-enabled", false, "Enable the Generic Webhook for submitting jobs")
	pflag.String("generic-webhook-secret", "", "Secret for validating incoming Generic Webhook POST events")

	// We need to parse --config early to load the config file before full flag parsing
	// so we can resolve aliases defined in the config.
	for i, arg := range os.Args {
		if (arg == "--config" || arg == "-c") && i+1 < len(os.Args) {
			cfgFile = os.Args[i+1]
			break
		} else if strings.HasPrefix(arg, "--config=") || strings.HasPrefix(arg, "-c=") {
			cfgFile = strings.SplitN(arg, "=", 2)[1]
			break
		}
	}

	// Load config early for aliases
	config.Load(cfgFile)

	// Alias resolution
	os.Args = expandAliases(os.Args)

	pflag.Parse()

	// Load config again after pflag.Parse to ensure any final flag values override correctly
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
	viper.BindPFlag("orchestrator.linear_webhook_secret", pflag.Lookup("linear-webhook-secret"))

	viper.BindPFlag("orchestrator.gitlab_token", pflag.Lookup("gitlab-token"))
	viper.BindPFlag("orchestrator.gitlab_project", pflag.Lookup("gitlab-project"))
	viper.BindPFlag("orchestrator.gitlab_label", pflag.Lookup("gitlab-label"))
	viper.BindPFlag("orchestrator.gitlab_url", pflag.Lookup("gitlab-url"))
	viper.BindPFlag("orchestrator.gitlab_webhook_secret", pflag.Lookup("gitlab-webhook-secret"))
	viper.BindPFlag("orchestrator.jira_webhook_secret", pflag.Lookup("jira-webhook-secret"))

	viper.BindPFlag("orchestrator.dry_run", pflag.Lookup("dry-run"))
	viper.BindEnv("orchestrator.dry_run", "RECAC_ORCHESTRATOR_DRY_RUN")

	viper.BindPFlag("orchestrator.verify", pflag.Lookup("verify"))
	viper.BindPFlag("orchestrator.list_jobs", pflag.Lookup("list-jobs"))
	viper.BindPFlag("orchestrator.list_pending", pflag.Lookup("list-pending"))
	viper.BindPFlag("orchestrator.list_tags", pflag.Lookup("list-tags"))
	viper.BindPFlag("orchestrator.list_groups", pflag.Lookup("list-groups"))
	viper.BindPFlag("orchestrator.tree", pflag.Lookup("tree"))
	viper.BindPFlag("orchestrator.tree_job", pflag.Lookup("tree-job"))
	viper.BindPFlag("orchestrator.timeline", pflag.Lookup("timeline"))
	viper.BindPFlag("orchestrator.timeline_limit", pflag.Lookup("timeline-limit"))
	viper.BindPFlag("orchestrator.history", pflag.Lookup("history"))
	viper.BindPFlag("orchestrator.list_dependents", pflag.Lookup("list-dependents"))
	viper.BindPFlag("orchestrator.list_blockers", pflag.Lookup("list-blockers"))
	viper.BindPFlag("orchestrator.list_jobs_status", pflag.Lookup("list-jobs-status"))
	viper.BindPFlag("orchestrator.list_jobs_tag", pflag.Lookup("list-jobs-tag"))
	viper.BindPFlag("orchestrator.list_jobs_match", pflag.Lookup("list-jobs-match"))
	viper.BindPFlag("orchestrator.list_jobs_priority", pflag.Lookup("list-jobs-priority"))
	viper.BindPFlag("orchestrator.search_jobs", pflag.Lookup("search-jobs"))
	viper.BindPFlag("orchestrator.list_jobs_format", pflag.Lookup("list-jobs-format"))
	viper.BindPFlag("orchestrator.format", pflag.Lookup("format"))
	viper.BindPFlag("orchestrator.summary", pflag.Lookup("summary"))
	viper.BindPFlag("orchestrator.watch", pflag.Lookup("watch"))
	viper.BindPFlag("orchestrator.watch_interval", pflag.Lookup("watch-interval"))
	viper.BindPFlag("orchestrator.status", pflag.Lookup("status"))
	viper.BindPFlag("orchestrator.tail_active", pflag.Lookup("tail-active"))
	viper.BindPFlag("orchestrator.tail_tag", pflag.Lookup("tail-tag"))
	viper.BindPFlag("orchestrator.tail_match", pflag.Lookup("tail-match"))
	viper.BindPFlag("orchestrator.tail_group", pflag.Lookup("tail-group"))
	viper.BindPFlag("orchestrator.tail_job", pflag.Lookup("tail-job"))
	viper.BindPFlag("orchestrator.analytics", pflag.Lookup("analytics"))
	viper.BindPFlag("orchestrator.critical_path", pflag.Lookup("critical-path"))
	viper.BindPFlag("orchestrator.monitor", pflag.Lookup("monitor"))
	viper.BindPFlag("orchestrator.stream_events", pflag.Lookup("stream-events"))
	viper.BindPFlag("orchestrator.logs", pflag.Lookup("logs"))
	viper.BindPFlag("orchestrator.edit_job", pflag.Lookup("edit-job"))
	viper.BindPFlag("orchestrator.build_job", pflag.Lookup("build-job"))
	viper.BindPFlag("orchestrator.inspect_job", pflag.Lookup("inspect-job"))
	viper.BindPFlag("orchestrator.inspect_dataflow", pflag.Lookup("inspect-dataflow"))
	viper.BindPFlag("orchestrator.explain_job", pflag.Lookup("explain-job"))
	viper.BindPFlag("orchestrator.explain_tag", pflag.Lookup("explain-tag"))
	viper.BindPFlag("orchestrator.explain_match", pflag.Lookup("explain-match"))
	viper.BindPFlag("orchestrator.analyze_failures", pflag.Lookup("analyze-failures"))
	viper.BindPFlag("orchestrator.analyze_durations", pflag.Lookup("analyze-durations"))
	viper.BindPFlag("orchestrator.analyze_durations_limit", pflag.Lookup("analyze-durations-limit"))
	viper.BindPFlag("orchestrator.analyze_reliability", pflag.Lookup("analyze-reliability"))
	viper.BindPFlag("orchestrator.analyze_reliability_limit", pflag.Lookup("analyze-reliability-limit"))
	viper.BindPFlag("orchestrator.analyze_costs", pflag.Lookup("analyze-costs"))
	viper.BindPFlag("orchestrator.analyze_costs_limit", pflag.Lookup("analyze-costs-limit"))
	viper.BindPFlag("orchestrator.analyze_anomalies", pflag.Lookup("analyze-anomalies"))
	viper.BindPFlag("orchestrator.analyze_anomalies_limit", pflag.Lookup("analyze-anomalies-limit"))
	viper.BindPFlag("orchestrator.analyze_agents", pflag.Lookup("analyze-agents"))
	viper.BindPFlag("orchestrator.analyze_agents_limit", pflag.Lookup("analyze-agents-limit"))
	viper.BindPFlag("orchestrator.analyze_tags", pflag.Lookup("analyze-tags"))
	viper.BindPFlag("orchestrator.analyze_tags_limit", pflag.Lookup("analyze-tags-limit"))
	viper.BindPFlag("orchestrator.heal_job", pflag.Lookup("heal-job"))
	viper.BindPFlag("orchestrator.heal_match", pflag.Lookup("heal-match"))
	viper.BindPFlag("orchestrator.heal_tag", pflag.Lookup("heal-tag"))
	viper.BindPFlag("orchestrator.compare_jobs", pflag.Lookup("compare-jobs"))
	viper.BindPFlag("orchestrator.cancel_job", pflag.Lookup("cancel-job"))
	viper.BindPFlag("orchestrator.cancel_all", pflag.Lookup("cancel-all"))
	viper.BindPFlag("orchestrator.cancel_tag", pflag.Lookup("cancel-tag"))
	viper.BindPFlag("orchestrator.cancel_status", pflag.Lookup("cancel-status"))
	viper.BindPFlag("orchestrator.cancel_match", pflag.Lookup("cancel-match"))
	viper.BindPFlag("orchestrator.cancel_group", pflag.Lookup("cancel-group"))
	viper.BindPFlag("orchestrator.cancel_older_than", pflag.Lookup("cancel-older-than"))
	viper.BindPFlag("orchestrator.pause_group", pflag.Lookup("pause-group"))
	viper.BindPFlag("orchestrator.resume_group", pflag.Lookup("resume-group"))
	viper.BindPFlag("orchestrator.purge_job", pflag.Lookup("purge-job"))
	viper.BindPFlag("orchestrator.purge_tag", pflag.Lookup("purge-tag"))
	viper.BindPFlag("orchestrator.purge_status", pflag.Lookup("purge-status"))
	viper.BindPFlag("orchestrator.purge_match", pflag.Lookup("purge-match"))
	viper.BindPFlag("orchestrator.purge_group", pflag.Lookup("purge-group"))
	viper.BindPFlag("orchestrator.purge_older_than", pflag.Lookup("purge-older-than"))
	viper.BindPFlag("orchestrator.purge_failed", pflag.Lookup("purge-failed"))
	viper.BindPFlag("orchestrator.clear_history", pflag.Lookup("clear-history"))
	viper.BindPFlag("orchestrator.clear_pending", pflag.Lookup("clear-pending"))
	viper.BindPFlag("orchestrator.clean_all", pflag.Lookup("clean-all"))
	viper.BindPFlag("orchestrator.delete_pending_job", pflag.Lookup("delete-pending-job"))
	viper.BindPFlag("orchestrator.delete_pending_tag", pflag.Lookup("delete-pending-tag"))
	viper.BindPFlag("orchestrator.delete_pending_match", pflag.Lookup("delete-pending-match"))
	viper.BindPFlag("orchestrator.delete_pending_group", pflag.Lookup("delete-pending-group"))
	viper.BindPFlag("orchestrator.delete_pending_older_than", pflag.Lookup("delete-pending-older-than"))
	viper.BindPFlag("orchestrator.retry_job", pflag.Lookup("retry-job"))
	viper.BindPFlag("orchestrator.downstream", pflag.Lookup("downstream"))
	viper.BindPFlag("orchestrator.retry_edit_job", pflag.Lookup("retry-edit-job"))
	viper.BindPFlag("orchestrator.clone_job", pflag.Lookup("clone-job"))
	viper.BindPFlag("orchestrator.clone_match", pflag.Lookup("clone-match"))
	viper.BindPFlag("orchestrator.clone_tag", pflag.Lookup("clone-tag"))
	viper.BindPFlag("orchestrator.clone_group", pflag.Lookup("clone-group"))
	viper.BindPFlag("orchestrator.clone_remap_deps", pflag.Lookup("clone-remap-deps"))
	viper.BindPFlag("orchestrator.retry_failed", pflag.Lookup("retry-failed"))
	viper.BindPFlag("orchestrator.retry_match", pflag.Lookup("retry-match"))
	viper.BindPFlag("orchestrator.retry_tag", pflag.Lookup("retry-tag"))
	viper.BindPFlag("orchestrator.retry_group", pflag.Lookup("retry-group"))
	viper.BindPFlag("orchestrator.require_approval", pflag.Lookup("require-approval"))
	viper.BindPFlag("orchestrator.approve_job", pflag.Lookup("approve-job"))
	viper.BindPFlag("orchestrator.approve_tag", pflag.Lookup("approve-tag"))
	viper.BindPFlag("orchestrator.approve_match", pflag.Lookup("approve-match"))
	viper.BindPFlag("orchestrator.approve_group", pflag.Lookup("approve-group"))
	viper.BindPFlag("orchestrator.approve_older_than", pflag.Lookup("approve-older-than"))
	viper.BindPFlag("orchestrator.approve_interactive", pflag.Lookup("approve-interactive"))
	viper.BindPFlag("orchestrator.hold_job", pflag.Lookup("hold-job"))
	viper.BindPFlag("orchestrator.unhold_job", pflag.Lookup("unhold-job"))
	viper.BindPFlag("orchestrator.hold_tag", pflag.Lookup("hold-tag"))
	viper.BindPFlag("orchestrator.hold_match", pflag.Lookup("hold-match"))
	viper.BindPFlag("orchestrator.hold_group", pflag.Lookup("hold-group"))
	viper.BindPFlag("orchestrator.unhold_tag", pflag.Lookup("unhold-tag"))
	viper.BindPFlag("orchestrator.unhold_match", pflag.Lookup("unhold-match"))
	viper.BindPFlag("orchestrator.unhold_group", pflag.Lookup("unhold-group"))
	viper.BindPFlag("orchestrator.rename_job", pflag.Lookup("rename-job"))
	viper.BindPFlag("orchestrator.new_job_id", pflag.Lookup("new-job-id"))
	viper.BindPFlag("orchestrator.skip_job", pflag.Lookup("skip-job"))
	viper.BindPFlag("orchestrator.skip_tag", pflag.Lookup("skip-tag"))
	viper.BindPFlag("orchestrator.skip_match", pflag.Lookup("skip-match"))
	viper.BindPFlag("orchestrator.skip_group", pflag.Lookup("skip-group"))
	viper.BindPFlag("orchestrator.skip_older_than", pflag.Lookup("skip-older-than"))
	viper.BindPFlag("orchestrator.force_complete_job", pflag.Lookup("force-complete-job"))
	viper.BindPFlag("orchestrator.force_complete_tag", pflag.Lookup("force-complete-tag"))
	viper.BindPFlag("orchestrator.force_complete_match", pflag.Lookup("force-complete-match"))
	viper.BindPFlag("orchestrator.fail_job", pflag.Lookup("fail-job"))
	viper.BindPFlag("orchestrator.fail_tag", pflag.Lookup("fail-tag"))
	viper.BindPFlag("orchestrator.fail_match", pflag.Lookup("fail-match"))
	viper.BindPFlag("orchestrator.fail_group", pflag.Lookup("fail-group"))
	viper.BindPFlag("orchestrator.diagnose", pflag.Lookup("diagnose"))
	viper.BindPFlag("orchestrator.simulate", pflag.Lookup("simulate"))
	viper.BindPFlag("orchestrator.simulate_pipeline", pflag.Lookup("simulate-pipeline"))
	viper.BindPFlag("orchestrator.simulate_pipeline_out", pflag.Lookup("simulate-pipeline-out"))
	viper.BindPFlag("orchestrator.pause", pflag.Lookup("pause"))
	viper.BindPFlag("orchestrator.resume", pflag.Lookup("resume"))
	viper.BindPFlag("orchestrator.drain", pflag.Lookup("drain"))
	viper.BindPFlag("orchestrator.undrain", pflag.Lookup("undrain"))
	viper.BindPFlag("orchestrator.force_poll", pflag.Lookup("force-poll"))
	viper.BindPFlag("orchestrator.scale", pflag.Lookup("scale"))
	viper.BindPFlag("orchestrator.update_interval", pflag.Lookup("update-interval"))
	viper.BindPFlag("orchestrator.update_priority", pflag.Lookup("update-priority"))
	viper.BindPFlag("orchestrator.update_priority_tag", pflag.Lookup("update-priority-tag"))
	viper.BindPFlag("orchestrator.update_priority_match", pflag.Lookup("update-priority-match"))
	viper.BindPFlag("orchestrator.update_priority_group", pflag.Lookup("update-priority-group"))
	viper.BindPFlag("orchestrator.priority_val", pflag.Lookup("priority-val"))
	viper.BindPFlag("orchestrator.promote_job", pflag.Lookup("promote-job"))
	viper.BindPFlag("orchestrator.promote_tag", pflag.Lookup("promote-tag"))
	viper.BindPFlag("orchestrator.promote_match", pflag.Lookup("promote-match"))
	viper.BindPFlag("orchestrator.promote_group", pflag.Lookup("promote-group"))
	viper.BindPFlag("orchestrator.demote_job", pflag.Lookup("demote-job"))
	viper.BindPFlag("orchestrator.demote_tag", pflag.Lookup("demote-tag"))
	viper.BindPFlag("orchestrator.demote_match", pflag.Lookup("demote-match"))
	viper.BindPFlag("orchestrator.demote_group", pflag.Lookup("demote-group"))
	viper.BindPFlag("orchestrator.update_timeout", pflag.Lookup("update-timeout"))
	viper.BindPFlag("orchestrator.update_timeout_tag", pflag.Lookup("update-timeout-tag"))
	viper.BindPFlag("orchestrator.update_timeout_match", pflag.Lookup("update-timeout-match"))
	viper.BindPFlag("orchestrator.timeout_val", pflag.Lookup("timeout-val"))
	viper.BindPFlag("orchestrator.update_max_retries_job", pflag.Lookup("update-max-retries-job"))
	viper.BindPFlag("orchestrator.update_max_retries_tag", pflag.Lookup("update-max-retries-tag"))
	viper.BindPFlag("orchestrator.update_max_retries_match", pflag.Lookup("update-max-retries-match"))
	viper.BindPFlag("orchestrator.max_retries_val", pflag.Lookup("max-retries-val"))
	viper.BindPFlag("orchestrator.update_agent_job", pflag.Lookup("update-agent-job"))
	viper.BindPFlag("orchestrator.update_agent_tag", pflag.Lookup("update-agent-tag"))
	viper.BindPFlag("orchestrator.update_agent_match", pflag.Lookup("update-agent-match"))
	viper.BindPFlag("orchestrator.agent_provider_val", pflag.Lookup("agent-provider-val"))
	viper.BindPFlag("orchestrator.agent_model_val", pflag.Lookup("agent-model-val"))
	viper.BindPFlag("orchestrator.set_progress_job", pflag.Lookup("set-progress-job"))
	viper.BindPFlag("orchestrator.progress_val", pflag.Lookup("progress-val"))
	viper.BindPFlag("orchestrator.progress_msg", pflag.Lookup("progress-msg"))
	viper.BindPFlag("orchestrator.update_deps_job", pflag.Lookup("update-deps-job"))
	viper.BindPFlag("orchestrator.update_deps_tag", pflag.Lookup("update-deps-tag"))
	viper.BindPFlag("orchestrator.update_deps_match", pflag.Lookup("update-deps-match"))
	viper.BindPFlag("orchestrator.set_deps", pflag.Lookup("set-deps"))
	viper.BindPFlag("orchestrator.update_env_job", pflag.Lookup("update-env-job"))
	viper.BindPFlag("orchestrator.update_env_tag", pflag.Lookup("update-env-tag"))
	viper.BindPFlag("orchestrator.update_env_match", pflag.Lookup("update-env-match"))
	viper.BindPFlag("orchestrator.set_env", pflag.Lookup("set-env"))
	viper.BindPFlag("orchestrator.update_tags_job", pflag.Lookup("update-tags-job"))
	viper.BindPFlag("orchestrator.update_tags_tag", pflag.Lookup("update-tags-tag"))
	viper.BindPFlag("orchestrator.update_tags_match", pflag.Lookup("update-tags-match"))

	viper.BindPFlag("orchestrator.add_tag_job", pflag.Lookup("add-tag-job"))
	viper.BindPFlag("orchestrator.add_tag_tag", pflag.Lookup("add-tag-tag"))
	viper.BindPFlag("orchestrator.add_tag_match", pflag.Lookup("add-tag-match"))
	viper.BindPFlag("orchestrator.remove_tag_job", pflag.Lookup("remove-tag-job"))
	viper.BindPFlag("orchestrator.remove_tag_tag", pflag.Lookup("remove-tag-tag"))
	viper.BindPFlag("orchestrator.remove_tag_match", pflag.Lookup("remove-tag-match"))

	viper.BindPFlag("orchestrator.set_tags", pflag.Lookup("set-tags"))
	viper.BindPFlag("orchestrator.wait_job", pflag.Lookup("wait-job"))
	viper.BindPFlag("orchestrator.wait_jobs", pflag.Lookup("wait-jobs"))
	viper.BindPFlag("orchestrator.wait_tag", pflag.Lookup("wait-tag"))
	viper.BindPFlag("orchestrator.wait_match", pflag.Lookup("wait-match"))
	viper.BindPFlag("orchestrator.wait_group", pflag.Lookup("wait-group"))
	viper.BindPFlag("orchestrator.wait_idle", pflag.Lookup("wait-idle"))
	viper.BindPFlag("orchestrator.set_output_job", pflag.Lookup("set-output-job"))
	viper.BindPFlag("orchestrator.set_output_key", pflag.Lookup("set-output-key"))
	viper.BindPFlag("orchestrator.set_output_val", pflag.Lookup("set-output-val"))
	viper.BindPFlag("orchestrator.get_output_job", pflag.Lookup("get-output-job"))
	viper.BindPFlag("orchestrator.get_output_key", pflag.Lookup("get-output-key"))
	viper.BindPFlag("orchestrator.get_metrics_job", pflag.Lookup("get-metrics-job"))
	viper.BindPFlag("orchestrator.get_metrics_key", pflag.Lookup("get-metrics-key"))
	viper.BindPFlag("orchestrator.add_metrics_job", pflag.Lookup("add-metrics-job"))
	viper.BindPFlag("orchestrator.metrics_key", pflag.Lookup("metrics-key"))
	viper.BindPFlag("orchestrator.metrics_val", pflag.Lookup("metrics-val"))
	viper.BindPFlag("orchestrator.archive_job", pflag.Lookup("archive-job"))
	viper.BindPFlag("orchestrator.archive_out", pflag.Lookup("archive-out"))
	viper.BindPFlag("orchestrator.archive_tag", pflag.Lookup("archive-tag"))
	viper.BindPFlag("orchestrator.archive_match", pflag.Lookup("archive-match"))
	viper.BindPFlag("orchestrator.archive_failed", pflag.Lookup("archive-failed"))
	viper.BindPFlag("orchestrator.archive_status", pflag.Lookup("archive-status"))
	viper.BindPFlag("orchestrator.archive_group", pflag.Lookup("archive-group"))
	viper.BindPFlag("orchestrator.archive_older_than", pflag.Lookup("archive-older-than"))
	viper.BindPFlag("orchestrator.submit", pflag.Lookup("submit"))
	viper.BindPFlag("orchestrator.submit_batch", pflag.Lookup("submit-batch"))
	viper.BindPFlag("orchestrator.submit_matrix", pflag.Lookup("submit-matrix"))
	viper.BindPFlag("orchestrator.submit_matrix_inline", pflag.Lookup("submit-matrix-inline"))
	viper.BindPFlag("orchestrator.submit_pipeline", pflag.Lookup("submit-pipeline"))
	viper.BindPFlag("orchestrator.validate_pipeline", pflag.Lookup("validate-pipeline"))
	viper.BindPFlag("orchestrator.submit_pipeline_target", pflag.Lookup("submit-pipeline-target"))
	viper.BindPFlag("orchestrator.pipeline_var", pflag.Lookup("pipeline-var"))
	viper.BindPFlag("orchestrator.pipeline_var_file", pflag.Lookup("pipeline-var-file"))
	viper.BindPFlag("orchestrator.submit_pipeline_interactive", pflag.Lookup("submit-pipeline-interactive"))
	viper.BindPFlag("orchestrator.dry_run_pipeline", pflag.Lookup("dry-run-pipeline"))
	viper.BindPFlag("orchestrator.lint_pipeline", pflag.Lookup("lint-pipeline"))
	viper.BindPFlag("orchestrator.import_pipeline", pflag.Lookup("import-pipeline"))
	viper.BindPFlag("orchestrator.explain_pipeline", pflag.Lookup("explain-pipeline"))
	viper.BindPFlag("orchestrator.export_pipeline_graph", pflag.Lookup("export-pipeline-graph"))
	viper.BindPFlag("orchestrator.export_pipeline_graph_format", pflag.Lookup("export-pipeline-graph-format"))
	viper.BindPFlag("orchestrator.export_pipeline_graph_out", pflag.Lookup("export-pipeline-graph-out"))
	viper.BindPFlag("orchestrator.list_templates", pflag.Lookup("list-templates"))
	viper.BindPFlag("orchestrator.list_pipeline_vars", pflag.Lookup("list-pipeline-vars"))
	viper.BindPFlag("orchestrator.inspect_pipeline", pflag.Lookup("inspect-pipeline"))
	viper.BindPFlag("orchestrator.compare_pipelines", pflag.Lookup("compare-pipelines"))
	viper.BindPFlag("orchestrator.apply_pipeline", pflag.Lookup("apply-pipeline"))
	viper.BindPFlag("orchestrator.watch_pipeline", pflag.Lookup("watch-pipeline"))
	viper.BindPFlag("orchestrator.watch_pipeline_interval", pflag.Lookup("watch-pipeline-interval"))
	viper.BindPFlag("orchestrator.apply_pipeline_dry_run", pflag.Lookup("apply-pipeline-dry-run"))
	viper.BindPFlag("orchestrator.apply_pipeline_run_id", pflag.Lookup("apply-pipeline-run-id"))
	viper.BindPFlag("orchestrator.search_logs", pflag.Lookup("search-logs"))
	viper.BindPFlag("orchestrator.search_tag", pflag.Lookup("search-tag"))
	viper.BindPFlag("orchestrator.search_status", pflag.Lookup("search-status"))
	viper.BindPFlag("orchestrator.search_context", pflag.Lookup("search-context"))
	viper.BindPFlag("orchestrator.submit_url", pflag.Lookup("submit-url"))
	viper.BindPFlag("orchestrator.submit_task", pflag.Lookup("submit-task"))
	viper.BindPFlag("orchestrator.submit_id", pflag.Lookup("submit-id"))
	viper.BindPFlag("orchestrator.submit_priority", pflag.Lookup("submit-priority"))
	viper.BindPFlag("orchestrator.submit_delay", pflag.Lookup("submit-delay"))
	viper.BindPFlag("orchestrator.env", pflag.Lookup("env"))
	viper.BindPFlag("orchestrator.submit_deps", pflag.Lookup("submit-deps"))
	viper.BindPFlag("orchestrator.submit_tags", pflag.Lookup("submit-tags"))
	viper.BindPFlag("orchestrator.submit_timeout", pflag.Lookup("submit-timeout"))
	viper.BindPFlag("orchestrator.submit_dependency_timeout", pflag.Lookup("submit-dependency-timeout"))
	viper.BindPFlag("orchestrator.submit_max_retries", pflag.Lookup("submit-max-retries"))
	viper.BindPFlag("orchestrator.submit_require_approval", pflag.Lookup("submit-require-approval"))
	viper.BindPFlag("orchestrator.submit_retry_delay", pflag.Lookup("submit-retry-delay"))
	viper.BindPFlag("orchestrator.submit_retry_backoff", pflag.Lookup("submit-retry-backoff"))
	viper.BindPFlag("orchestrator.submit_concurrency_group", pflag.Lookup("submit-concurrency-group"))
	viper.BindPFlag("orchestrator.submit_cancel_in_progress", pflag.Lookup("submit-cancel-in-progress"))
	viper.BindPFlag("orchestrator.submit_agent_provider", pflag.Lookup("submit-agent-provider"))
	viper.BindPFlag("orchestrator.submit_agent_model", pflag.Lookup("submit-agent-model"))
	viper.BindPFlag("orchestrator.submit_run_condition", pflag.Lookup("submit-run-condition"))
	viper.BindPFlag("orchestrator.submit_webhook_url", pflag.Lookup("submit-webhook-url"))
	viper.BindPFlag("orchestrator.submit_auto_heal", pflag.Lookup("submit-auto-heal"))
	viper.BindPFlag("orchestrator.wait", pflag.Lookup("wait"))
	viper.BindPFlag("orchestrator.host", pflag.Lookup("host"))

	viper.BindPFlag("orchestrator.export_job", pflag.Lookup("export-job"))
	viper.BindPFlag("orchestrator.export_job_out", pflag.Lookup("export-job-out"))
	viper.BindPFlag("orchestrator.export_jobs", pflag.Lookup("export-jobs"))
	viper.BindPFlag("orchestrator.import_jobs", pflag.Lookup("import-jobs"))
	viper.BindPFlag("orchestrator.export_format", pflag.Lookup("export-format"))
	viper.BindPFlag("orchestrator.export_pipeline", pflag.Lookup("export-pipeline"))
	viper.BindPFlag("orchestrator.export_graph", pflag.Lookup("export-graph"))
	viper.BindPFlag("orchestrator.export_graph_format", pflag.Lookup("export-graph-format"))
	viper.BindPFlag("orchestrator.export_metrics", pflag.Lookup("export-metrics"))
	viper.BindPFlag("orchestrator.export_metrics_state", pflag.Lookup("export-metrics-state"))
	viper.BindPFlag("orchestrator.export_junit", pflag.Lookup("export-junit"))
	viper.BindPFlag("orchestrator.export_trace", pflag.Lookup("export-trace"))
	viper.BindPFlag("orchestrator.export_trace_state", pflag.Lookup("export-trace-state"))
	viper.BindPFlag("orchestrator.export_timeline", pflag.Lookup("export-timeline"))
	viper.BindPFlag("orchestrator.export_timeline_state", pflag.Lookup("export-timeline-state"))
	viper.BindPFlag("orchestrator.export_costs", pflag.Lookup("export-costs"))
	viper.BindPFlag("orchestrator.export_costs_format", pflag.Lookup("export-costs-format"))
	viper.BindPFlag("orchestrator.export_agents", pflag.Lookup("export-agents"))
	viper.BindPFlag("orchestrator.export_agents_format", pflag.Lookup("export-agents-format"))
	viper.BindPFlag("orchestrator.export_durations", pflag.Lookup("export-durations"))
	viper.BindPFlag("orchestrator.export_durations_format", pflag.Lookup("export-durations-format"))
	viper.BindPFlag("orchestrator.export_reliability", pflag.Lookup("export-reliability"))
	viper.BindPFlag("orchestrator.export_reliability_format", pflag.Lookup("export-reliability-format"))
	viper.BindPFlag("orchestrator.export_failures", pflag.Lookup("export-failures"))
	viper.BindPFlag("orchestrator.export_failures_format", pflag.Lookup("export-failures-format"))
	viper.BindPFlag("orchestrator.export_anomalies", pflag.Lookup("export-anomalies"))
	viper.BindPFlag("orchestrator.export_anomalies_format", pflag.Lookup("export-anomalies-format"))
	viper.BindPFlag("orchestrator.export_tags", pflag.Lookup("export-tags"))
	viper.BindPFlag("orchestrator.export_tags_format", pflag.Lookup("export-tags-format"))

	viper.BindPFlag("orchestrator.upload_artifact", pflag.Lookup("upload-artifact"))
	viper.BindPFlag("orchestrator.download_artifact", pflag.Lookup("download-artifact"))
	viper.BindPFlag("orchestrator.artifact_out", pflag.Lookup("artifact-out"))
	viper.BindPFlag("orchestrator.list_artifacts", pflag.Lookup("list-artifacts"))
	viper.BindPFlag("orchestrator.delete_artifact", pflag.Lookup("delete-artifact"))
	viper.BindPFlag("orchestrator.job_id", pflag.Lookup("job-id"))

	viper.BindPFlag("orchestrator.generate_pipeline", pflag.Lookup("generate-pipeline"))
	viper.BindPFlag("orchestrator.generate_pipeline_out", pflag.Lookup("generate-pipeline-out"))

	viper.BindPFlag("orchestrator.generate_changelog", pflag.Lookup("generate-changelog"))
	viper.BindPFlag("orchestrator.changelog_tag", pflag.Lookup("changelog-tag"))
	viper.BindPFlag("orchestrator.changelog_match", pflag.Lookup("changelog-match"))

	viper.BindPFlag("orchestrator.generate_postmortem", pflag.Lookup("generate-postmortem"))
	viper.BindPFlag("orchestrator.postmortem_tag", pflag.Lookup("postmortem-tag"))
	viper.BindPFlag("orchestrator.postmortem_match", pflag.Lookup("postmortem-match"))

	viper.BindPFlag("orchestrator.optimize_pipeline", pflag.Lookup("optimize-pipeline"))
	viper.BindPFlag("orchestrator.optimize_pipeline_out", pflag.Lookup("optimize-pipeline-out"))

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
	viper.BindPFlag("orchestrator.max_budget", pflag.Lookup("max-budget"))
	viper.BindPFlag("orchestrator.log_dir", pflag.Lookup("log-dir"))
	viper.BindPFlag("orchestrator.artifacts_dir", pflag.Lookup("artifacts-dir"))
	viper.BindPFlag("orchestrator.retry_delay", pflag.Lookup("retry-delay"))
	viper.BindPFlag("orchestrator.circuit_breaker_max", pflag.Lookup("circuit-breaker-max"))

	viper.BindPFlag("orchestrator.cleanup", pflag.Lookup("cleanup"))
	viper.BindPFlag("orchestrator.cleanup_interval", pflag.Lookup("cleanup-interval"))
	viper.BindPFlag("orchestrator.cleanup_age", pflag.Lookup("cleanup-age"))

	viper.BindPFlag("orchestrator.trello_key", pflag.Lookup("trello-key"))
	viper.BindPFlag("orchestrator.trello_token", pflag.Lookup("trello-token"))
	viper.BindPFlag("orchestrator.trello_board", pflag.Lookup("trello-board"))
	viper.BindPFlag("orchestrator.trello_list", pflag.Lookup("trello-list"))
	viper.BindPFlag("orchestrator.trello_webhook_secret", pflag.Lookup("trello-webhook-secret"))

	viper.BindPFlag("orchestrator.asana_token", pflag.Lookup("asana-token"))
	viper.BindPFlag("orchestrator.asana_project", pflag.Lookup("asana-project"))

	viper.BindPFlag("orchestrator.notion_token", pflag.Lookup("notion-token"))
	viper.BindPFlag("orchestrator.notion_database_id", pflag.Lookup("notion-database-id"))
	viper.BindPFlag("orchestrator.notion_label", pflag.Lookup("notion-label"))

	viper.BindPFlag("orchestrator.cleanup_dry_run", pflag.Lookup("cleanup-dry-run"))

	viper.BindPFlag("orchestrator.generic_webhook_enabled", pflag.Lookup("generic-webhook-enabled"))
	viper.BindPFlag("orchestrator.generic_webhook_secret", pflag.Lookup("generic-webhook-secret"))
	viper.BindPFlag("orchestrator.allowed_pollers", pflag.Lookup("allowed-pollers"))

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
	viper.BindEnv("orchestrator.linear_webhook_secret", "RECAC_LINEAR_WEBHOOK_SECRET")
	viper.BindEnv("orchestrator.gitlab_token", "RECAC_GITLAB_TOKEN", "GITLAB_TOKEN")
	viper.BindEnv("orchestrator.gitlab_project", "RECAC_GITLAB_PROJECT")
	viper.BindEnv("orchestrator.gitlab_label", "RECAC_GITLAB_LABEL")
	viper.BindEnv("orchestrator.gitlab_url", "RECAC_GITLAB_URL")
	viper.BindEnv("orchestrator.gitlab_webhook_secret", "RECAC_GITLAB_WEBHOOK_SECRET")
	viper.BindEnv("orchestrator.jira_webhook_secret", "RECAC_JIRA_WEBHOOK_SECRET")
	viper.BindEnv("orchestrator.trello_key", "RECAC_TRELLO_KEY")
	viper.BindEnv("orchestrator.trello_token", "RECAC_TRELLO_TOKEN")
	viper.BindEnv("orchestrator.trello_board", "RECAC_TRELLO_BOARD")
	viper.BindEnv("orchestrator.trello_list", "RECAC_TRELLO_LIST")
	viper.BindEnv("orchestrator.trello_webhook_secret", "RECAC_TRELLO_WEBHOOK_SECRET")
	viper.BindEnv("orchestrator.asana_token", "RECAC_ASANA_TOKEN", "ASANA_TOKEN")
	viper.BindEnv("orchestrator.asana_project", "RECAC_ASANA_PROJECT")
	viper.BindEnv("orchestrator.notion_token", "RECAC_NOTION_TOKEN", "NOTION_TOKEN")
	viper.BindEnv("orchestrator.notion_database_id", "RECAC_NOTION_DATABASE_ID", "NOTION_DATABASE_ID")
	viper.BindEnv("orchestrator.notion_label", "RECAC_NOTION_LABEL", "NOTION_LABEL")
	viper.BindEnv("orchestrator.generic_webhook_enabled", "RECAC_GENERIC_WEBHOOK_ENABLED")
	viper.BindEnv("orchestrator.generic_webhook_secret", "RECAC_GENERIC_WEBHOOK_SECRET")
	viper.BindEnv("orchestrator.allowed_pollers", "RECAC_ALLOWED_POLLERS")
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
	viper.BindEnv("orchestrator.max_budget", "RECAC_MAX_BUDGET")
	viper.BindEnv("orchestrator.log_dir", "RECAC_LOG_DIR")
	viper.BindEnv("orchestrator.artifacts_dir", "RECAC_ARTIFACTS_DIR")
	viper.BindEnv("orchestrator.retry_delay", "RECAC_RETRY_DELAY")
	viper.BindEnv("orchestrator.circuit_breaker_max", "RECAC_CIRCUIT_BREAKER_MAX")
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

func loadPipelineVars(varList []string, varFile string) (map[string]string, error) {
	vars := make(map[string]string)

	if varFile != "" {
		ext := filepath.Ext(varFile)
		content, err := os.ReadFile(varFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read variable file: %w", err)
		}

		if strings.EqualFold(ext, ".json") {
			if err := json.Unmarshal(content, &vars); err != nil {
				return nil, fmt.Errorf("failed to parse JSON variable file: %w", err)
			}
		} else if strings.EqualFold(ext, ".yaml") || strings.EqualFold(ext, ".yml") {
			if err := yaml.Unmarshal(content, &vars); err != nil {
				return nil, fmt.Errorf("failed to parse YAML variable file: %w", err)
			}
		} else if strings.EqualFold(ext, ".env") || ext == "" {
			envVars, err := godotenv.Unmarshal(string(content))
			if err != nil {
				return nil, fmt.Errorf("failed to parse .env variable file: %w", err)
			}
			for k, v := range envVars {
				vars[k] = v
			}
		} else {
			return nil, fmt.Errorf("unsupported variable file format (must be .json, .yaml, .yml, or .env): %s", ext)
		}
	}

	for _, v := range varList {
		parts := strings.SplitN(v, "=", 2)
		if len(parts) == 2 {
			vars[parts[0]] = parts[1]
		}
	}

	return vars, nil
}

func run(ctx context.Context, logger *slog.Logger) error {
	if viper.GetBool("orchestrator.list_jobs") {
		host := viper.GetString("orchestrator.host")
		history := viper.GetBool("orchestrator.history")
		statusFilter := viper.GetString("orchestrator.list_jobs_status")
		tagFilter := viper.GetString("orchestrator.list_jobs_tag")
		matchFilter := viper.GetString("orchestrator.list_jobs_match")
		priorityFilter := viper.GetString("orchestrator.list_jobs_priority")
		format := viper.GetString("orchestrator.list_jobs_format")
		watch := viper.GetBool("orchestrator.watch")
		watchInterval := viper.GetDuration("orchestrator.watch_interval")

		if watch {
			ticker := time.NewTicker(watchInterval)
			defer ticker.Stop()
			for {
				fmt.Fprint(stdout, "\033[H\033[2J") // Clear screen
				listJobs(host, history, statusFilter, tagFilter, matchFilter, priorityFilter, format)
				select {
				case <-ctx.Done():
					return nil
				case <-ticker.C:
				}
			}
		} else {
			listJobs(host, history, statusFilter, tagFilter, matchFilter, priorityFilter, format)
		}
		return nil
	}

	if tailJobID := viper.GetString("orchestrator.tail_job"); tailJobID != "" {
		host := viper.GetString("orchestrator.host")
		if err := tailSingleJob(ctx, host, tailJobID); err != nil {
			fmt.Fprintf(stdout, "Tail failed: %v\n", err)
			exitFunc(1)
		}
		return nil
	}

	if viper.GetBool("orchestrator.wait_idle") {
		host := viper.GetString("orchestrator.host")
		if err := waitIdle(host, stdout); err != nil {
			fmt.Fprintf(stdout, "Wait idle failed: %v\n", err)
			exitFunc(1)
		}
		return nil
	}

	if waitJobsStr := viper.GetString("orchestrator.wait_jobs"); waitJobsStr != "" {
		host := viper.GetString("orchestrator.host")
		var jobIDs []string
		for _, id := range strings.Split(waitJobsStr, ",") {
			trimmed := strings.TrimSpace(id)
			if trimmed != "" {
				jobIDs = append(jobIDs, trimmed)
			}
		}

		if len(jobIDs) > 0 {
			if err := waitForJobs(host, jobIDs, stdout); err != nil {
				fmt.Fprintf(stdout, "Wait for jobs failed: %v\n", err)
				exitFunc(1)
			}
		} else {
			fmt.Fprintf(stdout, "No valid job IDs provided to --wait-jobs\n")
			exitFunc(1)
		}
		return nil
	}

	if viper.GetBool("orchestrator.list_tags") {
		host := viper.GetString("orchestrator.host")
		listTags(host)
		return nil
	}

	if viper.GetBool("orchestrator.list_groups") {
		host := viper.GetString("orchestrator.host")
		format := viper.GetString("orchestrator.format")
		listGroups(host, format)
		return nil
	}

	if query := viper.GetString("orchestrator.search_jobs"); query != "" {
		host := viper.GetString("orchestrator.host")
		tag := viper.GetString("orchestrator.list_jobs_tag")
		status := viper.GetString("orchestrator.list_jobs_status")
		format := viper.GetString("orchestrator.list_jobs_format")
		searchJobsGlobally(host, query, tag, status, format)
		return nil
	}

	if viper.GetBool("orchestrator.list_pending") {
		host := viper.GetString("orchestrator.host")
		format := viper.GetString("orchestrator.list_jobs_format")
		priorityFilter := viper.GetString("orchestrator.list_jobs_priority")
		watch := viper.GetBool("orchestrator.watch")
		watchInterval := viper.GetDuration("orchestrator.watch_interval")

		if watch {
			ticker := time.NewTicker(watchInterval)
			defer ticker.Stop()
			for {
				fmt.Fprint(stdout, "\033[H\033[2J") // Clear screen
				listPendingJobs(host, priorityFilter, format)
				select {
				case <-ctx.Done():
					return nil
				case <-ticker.C:
				}
			}
		} else {
			listPendingJobs(host, priorityFilter, format)
		}
		return nil
	}

	if listDependentsID := viper.GetString("orchestrator.list_dependents"); listDependentsID != "" {
		host := viper.GetString("orchestrator.host")
		listDependents(host, listDependentsID)
		return nil
	}

	if listBlockersID := viper.GetString("orchestrator.list_blockers"); listBlockersID != "" {
		host := viper.GetString("orchestrator.host")
		listBlockers(host, listBlockersID)
		return nil
	}

	if viper.GetBool("orchestrator.summary") {
		host := viper.GetString("orchestrator.host")
		format := viper.GetString("orchestrator.format")
		summaryJobs(host, format)
		return nil
	}

	if viper.GetBool("orchestrator.status") {
		host := viper.GetString("orchestrator.host")
		format := viper.GetString("orchestrator.format")
		printStatus(host, format)
		return nil
	}

	tailActive := viper.GetBool("orchestrator.tail_active")
	tailTag := viper.GetString("orchestrator.tail_tag")
	tailMatch := viper.GetString("orchestrator.tail_match")
	tailGroup := viper.GetString("orchestrator.tail_group")

	if tailActive || tailTag != "" || tailMatch != "" || tailGroup != "" {
		host := viper.GetString("orchestrator.host")
		if err := tailActiveJobs(ctx, host, tailTag, tailMatch, tailGroup); err != nil {
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

	if getOutputJob := viper.GetString("orchestrator.get_output_job"); getOutputJob != "" {
		host := viper.GetString("orchestrator.host")
		key := viper.GetString("orchestrator.get_output_key")
		if key == "" {
			fmt.Fprintf(stdout, "Error: --get-output-key is required when using --get-output-job\n")
			exitFunc(1)
			return nil
		}
		getJobOutput(host, getOutputJob, key)
		return nil
	}

	if getMetricsJob := viper.GetString("orchestrator.get_metrics_job"); getMetricsJob != "" {
		host := viper.GetString("orchestrator.host")
		key := viper.GetString("orchestrator.get_metrics_key")
		if key == "" {
			fmt.Fprintf(stdout, "Error: --get-metrics-key is required when using --get-metrics-job\n")
			exitFunc(1)
			return nil
		}
		getJobMetrics(host, getMetricsJob, key)
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

	if archiveJobId := viper.GetString("orchestrator.archive_job"); archiveJobId != "" {
		host := viper.GetString("orchestrator.host")
		outPath := viper.GetString("orchestrator.archive_out")
		archiveJob(host, archiveJobId, outPath)
		return nil
	}

	if archiveTag := viper.GetString("orchestrator.archive_tag"); archiveTag != "" {
		host := viper.GetString("orchestrator.host")
		outPath := viper.GetString("orchestrator.archive_out")
		archiveBulkJobs(host, archiveTag, "", "", "", "", outPath)
		return nil
	}

	if archiveMatch := viper.GetString("orchestrator.archive_match"); archiveMatch != "" {
		host := viper.GetString("orchestrator.host")
		outPath := viper.GetString("orchestrator.archive_out")
		archiveBulkJobs(host, "", archiveMatch, "", "", "", outPath)
		return nil
	}

	if viper.GetBool("orchestrator.archive_failed") {
		host := viper.GetString("orchestrator.host")
		outPath := viper.GetString("orchestrator.archive_out")
		archiveBulkJobs(host, "", "", "Failed", "", "", outPath)
		return nil
	}

	if archiveStatus := viper.GetString("orchestrator.archive_status"); archiveStatus != "" {
		host := viper.GetString("orchestrator.host")
		outPath := viper.GetString("orchestrator.archive_out")
		archiveBulkJobs(host, "", "", archiveStatus, "", "", outPath)
		return nil
	}

	if archiveGroup := viper.GetString("orchestrator.archive_group"); archiveGroup != "" {
		host := viper.GetString("orchestrator.host")
		outPath := viper.GetString("orchestrator.archive_out")
		archiveBulkJobs(host, "", "", "", archiveGroup, "", outPath)
		return nil
	}

	if archiveOlderThan := viper.GetString("orchestrator.archive_older_than"); archiveOlderThan != "" {
		host := viper.GetString("orchestrator.host")
		outPath := viper.GetString("orchestrator.archive_out")
		archiveBulkJobs(host, "", "", "", "", archiveOlderThan, outPath)
		return nil
	}

	if viper.GetBool("orchestrator.analytics") {
		host := viper.GetString("orchestrator.host")
		format := viper.GetString("orchestrator.format")
		printAnalytics(host, format)
		return nil
	}

	if viper.GetBool("orchestrator.critical_path") {
		host := viper.GetString("orchestrator.host")
		printCriticalPath(host)
		return nil
	}

	if treeJobID := viper.GetString("orchestrator.tree_job"); treeJobID != "" {
		host := viper.GetString("orchestrator.host")
		printJobTree(host, treeJobID)
		return nil
	}

	if viper.GetBool("orchestrator.tree") {
		host := viper.GetString("orchestrator.host")
		printTree(host)
		return nil
	}

	if viper.GetBool("orchestrator.timeline") {
		host := viper.GetString("orchestrator.host")
		limit := viper.GetInt("orchestrator.timeline_limit")
		printTimeline(host, limit)
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

	if viper.GetBool("orchestrator.build_job") {
		host := viper.GetString("orchestrator.host")
		wait := viper.GetBool("orchestrator.wait")
		buildJobInteractive(host, wait)
		return nil
	}

	if jobID := viper.GetString("orchestrator.inspect_job"); jobID != "" {
		host := viper.GetString("orchestrator.host")
		inspectJob(host, jobID)
		return nil
	}

	if jobID := viper.GetString("orchestrator.inspect_dataflow"); jobID != "" {
		host := viper.GetString("orchestrator.host")
		inspectDataflow(host, jobID)
		return nil
	}

	explainJobId := viper.GetString("orchestrator.explain_job")
	explainTag := viper.GetString("orchestrator.explain_tag")
	explainMatch := viper.GetString("orchestrator.explain_match")

	explainFlagsSet := 0
	if explainJobId != "" {
		explainFlagsSet++
	}
	if explainTag != "" {
		explainFlagsSet++
	}
	if explainMatch != "" {
		explainFlagsSet++
	}

	if explainFlagsSet > 1 {
		fmt.Fprintf(stdout, "Error: Cannot use --explain-job, --explain-tag, and --explain-match together. Please specify only one.\n")
		exitFunc(1)
		return nil
	}

	if explainJobId != "" {
		host := viper.GetString("orchestrator.host")
		provider := viper.GetString("orchestrator.agent_provider")
		model := viper.GetString("orchestrator.agent_model")
		explainJob(host, explainJobId, provider, model)
		return nil
	}

	if explainTag != "" || explainMatch != "" {
		host := viper.GetString("orchestrator.host")
		provider := viper.GetString("orchestrator.agent_provider")
		model := viper.GetString("orchestrator.agent_model")
		explainBulkJobs(host, explainMatch, explainTag, provider, model)
		return nil
	}

	if viper.GetBool("orchestrator.analyze_failures") {
		host := viper.GetString("orchestrator.host")
		analyzeFailures(host)
		return nil
	}

	if viper.GetBool("orchestrator.analyze_durations") {
		host := viper.GetString("orchestrator.host")
		limit := viper.GetInt("orchestrator.analyze_durations_limit")
		analyzeDurations(host, limit)
		return nil
	}

	if viper.GetBool("orchestrator.analyze_reliability") {
		host := viper.GetString("orchestrator.host")
		limit := viper.GetInt("orchestrator.analyze_reliability_limit")
		format := viper.GetString("orchestrator.format")
		analyzeReliability(host, limit, format)
		return nil
	}

	if viper.GetBool("orchestrator.analyze_costs") {
		host := viper.GetString("orchestrator.host")
		limit := viper.GetInt("orchestrator.analyze_costs_limit")
		format := viper.GetString("orchestrator.format")
		analyzeCosts(host, limit, format)
		return nil
	}

	if viper.GetBool("orchestrator.analyze_anomalies") {
		host := viper.GetString("orchestrator.host")
		limit := viper.GetInt("orchestrator.analyze_anomalies_limit")
		format := viper.GetString("orchestrator.format")
		analyzeAnomalies(host, limit, format)
		return nil
	}

	if viper.GetBool("orchestrator.analyze_tags") {
		host := viper.GetString("orchestrator.host")
		limit := viper.GetInt("orchestrator.analyze_tags_limit")
		format := viper.GetString("orchestrator.format")
		analyzeTags(host, limit, format)
		return nil
	}

	if viper.GetBool("orchestrator.analyze_agents") {
		host := viper.GetString("orchestrator.host")
		limit := viper.GetInt("orchestrator.analyze_agents_limit")
		format := viper.GetString("orchestrator.format")
		analyzeAgents(host, limit, format)
		return nil
	}

	if jobID := viper.GetString("orchestrator.heal_job"); jobID != "" {
		host := viper.GetString("orchestrator.host")
		wait := viper.GetBool("orchestrator.wait")
		healJob(host, jobID, wait)
		return nil
	}

	healMatch := viper.GetString("orchestrator.heal_match")
	healTag := viper.GetString("orchestrator.heal_tag")
	if healMatch != "" || healTag != "" {
		host := viper.GetString("orchestrator.host")
		healBulkJobs(host, healMatch, healTag)
		return nil
	}

	if compareJobsIds := viper.GetString("orchestrator.compare_jobs"); compareJobsIds != "" {
		host := viper.GetString("orchestrator.host")
		compareJobs(host, compareJobsIds)
		return nil
	}

	if jobID := viper.GetString("orchestrator.cancel_job"); jobID != "" {
		host := viper.GetString("orchestrator.host")
		downstream := viper.GetBool("orchestrator.downstream")
		cancelJob(host, jobID, downstream)
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

	if cancelGroup := viper.GetString("orchestrator.cancel_group"); cancelGroup != "" {
		host := viper.GetString("orchestrator.host")
		cancelJobsByGroup(host, cancelGroup)
		return nil
	}
	if exportDurationsFile := viper.GetString("orchestrator.export_durations"); exportDurationsFile != "" {
		host := viper.GetString("orchestrator.host")
		format := viper.GetString("orchestrator.export_durations_format")
		exportDurations(host, exportDurationsFile, format)
		return nil
	}
	if exportReliabilityFile := viper.GetString("orchestrator.export_reliability"); exportReliabilityFile != "" {
		host := viper.GetString("orchestrator.host")
		format := viper.GetString("orchestrator.export_reliability_format")
		exportReliability(host, exportReliabilityFile, format)
		return nil
	}
	if exportFailuresFile := viper.GetString("orchestrator.export_failures"); exportFailuresFile != "" {
		host := viper.GetString("orchestrator.host")
		format := viper.GetString("orchestrator.export_failures_format")
		exportFailures(host, exportFailuresFile, format)
		return nil
	}

	if cancelOlderThanStr := viper.GetString("orchestrator.cancel_older_than"); cancelOlderThanStr != "" {
		host := viper.GetString("orchestrator.host")
		cancelJobsOlderThan(host, cancelOlderThanStr)
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

	if purgeGroup := viper.GetString("orchestrator.purge_group"); purgeGroup != "" {
		host := viper.GetString("orchestrator.host")
		purgeJobsByGroup(host, purgeGroup)
		return nil
	}

	if purgeOlderThan := viper.GetString("orchestrator.purge_older_than"); purgeOlderThan != "" {
		host := viper.GetString("orchestrator.host")
		purgeJobsOlderThan(host, purgeOlderThan)
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

	if viper.GetBool("orchestrator.clean_all") {
		host := viper.GetString("orchestrator.host")
		cancelAllJobs(host)
		clearPending(host)
		clearHistory(host)
		return nil
	}

	if jobID := viper.GetString("orchestrator.delete_pending_job"); jobID != "" {
		host := viper.GetString("orchestrator.host")
		deletePendingJob(host, jobID)
		return nil
	}

	if deletePendingTag := viper.GetString("orchestrator.delete_pending_tag"); deletePendingTag != "" {
		host := viper.GetString("orchestrator.host")
		deletePendingJobsByTag(host, deletePendingTag)
		return nil
	}

	if deletePendingMatch := viper.GetString("orchestrator.delete_pending_match"); deletePendingMatch != "" {
		host := viper.GetString("orchestrator.host")
		deletePendingJobsByMatch(host, deletePendingMatch)
		return nil
	}

	if deletePendingGroup := viper.GetString("orchestrator.delete_pending_group"); deletePendingGroup != "" {
		host := viper.GetString("orchestrator.host")
		deletePendingJobsByGroup(host, deletePendingGroup)
		return nil
	}

	if deletePendingOlderThan := viper.GetString("orchestrator.delete_pending_older_than"); deletePendingOlderThan != "" {
		host := viper.GetString("orchestrator.host")
		deletePendingJobsOlderThan(host, deletePendingOlderThan)
		return nil
	}

	if jobID := viper.GetString("orchestrator.retry_job"); jobID != "" {
		host := viper.GetString("orchestrator.host")
		downstream := viper.GetBool("orchestrator.downstream")
		envPairs := viper.GetStringSlice("orchestrator.env")
		provider := viper.GetString("orchestrator.submit_agent_provider")
		model := viper.GetString("orchestrator.submit_agent_model")

		envMap := make(map[string]string)
		for _, pair := range envPairs {
			parts := strings.SplitN(pair, "=", 2)
			if len(parts) == 2 {
				envMap[parts[0]] = parts[1]
			} else {
				logger.Warn("Invalid environment variable format", "input", pair)
			}
		}

		retryJob(host, jobID, downstream, envMap, provider, model)
		return nil
	}

	if jobID := viper.GetString("orchestrator.retry_edit_job"); jobID != "" {
		host := viper.GetString("orchestrator.host")
		wait := viper.GetBool("orchestrator.wait")
		retryEditJob(host, jobID, wait)
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

	cloneMatch := viper.GetString("orchestrator.clone_match")
	cloneTag := viper.GetString("orchestrator.clone_tag")
	cloneGroup := viper.GetString("orchestrator.clone_group")
	if cloneMatch != "" || cloneTag != "" || cloneGroup != "" {
		host := viper.GetString("orchestrator.host")
		priority := viper.GetInt("orchestrator.submit_priority")
		wait := viper.GetBool("orchestrator.wait")
		remapDeps := viper.GetBool("orchestrator.clone_remap_deps")
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

		cloneBulkJobs(host, cloneMatch, cloneTag, cloneGroup, priorityPtr, wait, envMap, submitDepsPtr, remapDeps)
		return nil
	}

	retryMatch := viper.GetString("orchestrator.retry_match")
	retryTag := viper.GetString("orchestrator.retry_tag")
	retryGroup := viper.GetString("orchestrator.retry_group")
	if viper.GetBool("orchestrator.retry_failed") || retryMatch != "" || retryTag != "" || retryGroup != "" {
		host := viper.GetString("orchestrator.host")
		retryFailedJobs(host, retryMatch, retryTag, retryGroup)
		return nil
	}

	if approveJobId := viper.GetString("orchestrator.approve_job"); approveJobId != "" {
		host := viper.GetString("orchestrator.host")
		approveJob(host, approveJobId)
		return nil
	}

	if viper.GetBool("orchestrator.approve_interactive") {
		host := viper.GetString("orchestrator.host")
		approveInteractive(host)
		return nil
	}

	approveMatch := viper.GetString("orchestrator.approve_match")
	approveTag := viper.GetString("orchestrator.approve_tag")
	approveGroup := viper.GetString("orchestrator.approve_group")
	approveOlderThan := viper.GetString("orchestrator.approve_older_than")
	if approveMatch != "" || approveTag != "" || approveGroup != "" || approveOlderThan != "" {
		host := viper.GetString("orchestrator.host")
		approveBulkJobs(host, approveMatch, approveTag, approveGroup, approveOlderThan)
		return nil
	}

	if holdJobID := viper.GetString("orchestrator.hold_job"); holdJobID != "" {
		host := viper.GetString("orchestrator.host")
		holdJob(host, holdJobID)
		return nil
	}

	holdTag := viper.GetString("orchestrator.hold_tag")
	holdMatch := viper.GetString("orchestrator.hold_match")
	holdGroup := viper.GetString("orchestrator.hold_group")
	if holdTag != "" || holdMatch != "" || holdGroup != "" {
		host := viper.GetString("orchestrator.host")
		holdJobs(host, holdMatch, holdTag, holdGroup)
		return nil
	}

	if unholdJobID := viper.GetString("orchestrator.unhold_job"); unholdJobID != "" {
		host := viper.GetString("orchestrator.host")
		unholdJob(host, unholdJobID)
		return nil
	}

	unholdTag := viper.GetString("orchestrator.unhold_tag")
	unholdMatch := viper.GetString("orchestrator.unhold_match")
	unholdGroup := viper.GetString("orchestrator.unhold_group")
	if unholdTag != "" || unholdMatch != "" || unholdGroup != "" {
		host := viper.GetString("orchestrator.host")
		unholdJobs(host, unholdMatch, unholdTag, unholdGroup)
		return nil
	}

	if renameJobID := viper.GetString("orchestrator.rename_job"); renameJobID != "" {
		host := viper.GetString("orchestrator.host")
		newJobID := viper.GetString("orchestrator.new_job_id")
		if newJobID == "" {
			fmt.Fprintf(stdout, "Error: --new-job-id is required when using --rename-job\n")
			exitFunc(1)
			return nil
		}
		renameJob(host, renameJobID, newJobID)
		return nil
	}

	if skipJobID := viper.GetString("orchestrator.skip_job"); skipJobID != "" {
		host := viper.GetString("orchestrator.host")
		downstream := viper.GetBool("orchestrator.downstream")
		skipJob(host, skipJobID, downstream)
		return nil
	}

	skipTag := viper.GetString("orchestrator.skip_tag")
	skipMatch := viper.GetString("orchestrator.skip_match")
	skipGroup := viper.GetString("orchestrator.skip_group")
	if skipTag != "" || skipMatch != "" || skipGroup != "" {
		host := viper.GetString("orchestrator.host")
		skipJobs(host, skipMatch, skipTag, skipGroup)
		return nil
	}

	if skipOlderThanStr := viper.GetString("orchestrator.skip_older_than"); skipOlderThanStr != "" {
		host := viper.GetString("orchestrator.host")
		skipJobsOlderThan(host, skipOlderThanStr)
		return nil
	}

	if forceCompleteJobID := viper.GetString("orchestrator.force_complete_job"); forceCompleteJobID != "" {
		host := viper.GetString("orchestrator.host")
		forceCompleteJob(host, forceCompleteJobID)
		return nil
	}

	forceCompleteTag := viper.GetString("orchestrator.force_complete_tag")
	forceCompleteMatch := viper.GetString("orchestrator.force_complete_match")
	if forceCompleteTag != "" || forceCompleteMatch != "" {
		host := viper.GetString("orchestrator.host")
		forceCompleteBulkJobs(host, forceCompleteMatch, forceCompleteTag)
		return nil
	}

	if failJobID := viper.GetString("orchestrator.fail_job"); failJobID != "" {
		host := viper.GetString("orchestrator.host")
		failJob(host, failJobID)
		return nil
	}

	failTag := viper.GetString("orchestrator.fail_tag")
	failMatch := viper.GetString("orchestrator.fail_match")
	failGroup := viper.GetString("orchestrator.fail_group")
	if failTag != "" || failMatch != "" || failGroup != "" {
		host := viper.GetString("orchestrator.host")
		failBulkJobs(host, failMatch, failTag, failGroup)
		return nil
	}

	if viper.GetBool("orchestrator.diagnose") {
		host := viper.GetString("orchestrator.host")
		runDiagnose(host)
		return nil
	}

	if viper.GetBool("orchestrator.simulate") {
		host := viper.GetString("orchestrator.host")
		simulateExecution(host)
		return nil
	}

	if simulatePipelineFile := viper.GetString("orchestrator.simulate_pipeline"); simulatePipelineFile != "" {
		host := viper.GetString("orchestrator.host")
		target := viper.GetString("orchestrator.submit_pipeline_target")
		outFile := viper.GetString("orchestrator.simulate_pipeline_out")

		simulatePipelineFileCmd(host, simulatePipelineFile, target, outFile)
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

	if pauseGroup := viper.GetString("orchestrator.pause_group"); pauseGroup != "" {
		host := viper.GetString("orchestrator.host")
		pauseOrchestratorGroup(host, pauseGroup)
		return nil
	}

	if resumeGroup := viper.GetString("orchestrator.resume_group"); resumeGroup != "" {
		host := viper.GetString("orchestrator.host")
		resumeOrchestratorGroup(host, resumeGroup)
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

	if updateIntervalVal := viper.GetString("orchestrator.update_interval"); updateIntervalVal != "" {
		host := viper.GetString("orchestrator.host")
		updatePollInterval(host, updateIntervalVal)
		return nil
	}

	updatePriorityJob := viper.GetString("orchestrator.update_priority")
	updatePriorityTag := viper.GetString("orchestrator.update_priority_tag")
	updatePriorityMatch := viper.GetString("orchestrator.update_priority_match")
	updatePriorityGroup := viper.GetString("orchestrator.update_priority_group")

	priorityFlagsSet := 0
	if updatePriorityJob != "" {
		priorityFlagsSet++
	}
	if updatePriorityTag != "" {
		priorityFlagsSet++
	}
	if updatePriorityMatch != "" {
		priorityFlagsSet++
	}
	if updatePriorityGroup != "" {
		priorityFlagsSet++
	}

	if priorityFlagsSet > 1 {
		fmt.Fprintf(stdout, "Error: Cannot use --update-priority, --update-priority-tag, --update-priority-match, and --update-priority-group together. Please specify only one.\n")
		exitFunc(1)
		return nil
	}

	if priorityFlagsSet > 0 {
		host := viper.GetString("orchestrator.host")
		priorityVal := viper.GetInt("orchestrator.priority_val")

		if updatePriorityJob != "" {
			updatePriority(host, updatePriorityJob, priorityVal)
		} else {
			updateBulkPriority(host, updatePriorityMatch, updatePriorityTag, updatePriorityGroup, priorityVal)
		}
		return nil
	}

	promoteJobID := viper.GetString("orchestrator.promote_job")
	promoteTag := viper.GetString("orchestrator.promote_tag")
	promoteMatch := viper.GetString("orchestrator.promote_match")
	promoteGroup := viper.GetString("orchestrator.promote_group")

	promoteFlagsSet := 0
	if promoteJobID != "" {
		promoteFlagsSet++
	}
	if promoteTag != "" {
		promoteFlagsSet++
	}
	if promoteMatch != "" {
		promoteFlagsSet++
	}
	if promoteGroup != "" {
		promoteFlagsSet++
	}

	if promoteFlagsSet > 1 {
		fmt.Fprintf(stdout, "Error: Cannot use --promote-job, --promote-tag, --promote-match, and --promote-group together. Please specify only one.\n")
		exitFunc(1)
		return nil
	}

	if promoteJobID != "" {
		host := viper.GetString("orchestrator.host")
		promoteJob(host, promoteJobID)
		return nil
	}

	if promoteTag != "" || promoteMatch != "" || promoteGroup != "" {
		host := viper.GetString("orchestrator.host")
		promoteBulkJobs(host, promoteMatch, promoteTag, promoteGroup)
		return nil
	}

	demoteJobID := viper.GetString("orchestrator.demote_job")
	demoteTag := viper.GetString("orchestrator.demote_tag")
	demoteMatch := viper.GetString("orchestrator.demote_match")
	demoteGroup := viper.GetString("orchestrator.demote_group")

	demoteFlagsSet := 0
	if demoteJobID != "" {
		demoteFlagsSet++
	}
	if demoteTag != "" {
		demoteFlagsSet++
	}
	if demoteMatch != "" {
		demoteFlagsSet++
	}
	if demoteGroup != "" {
		demoteFlagsSet++
	}

	if demoteFlagsSet > 1 {
		fmt.Fprintf(stdout, "Error: Cannot use --demote-job, --demote-tag, --demote-match, and --demote-group together. Please specify only one.\n")
		exitFunc(1)
		return nil
	}

	if demoteJobID != "" {
		host := viper.GetString("orchestrator.host")
		demoteJob(host, demoteJobID)
		return nil
	}

	if demoteTag != "" || demoteMatch != "" || demoteGroup != "" {
		host := viper.GetString("orchestrator.host")
		demoteBulkJobs(host, demoteMatch, demoteTag, demoteGroup)
		return nil
	}

	if updatePriorityTag != "" || updatePriorityMatch != "" || updatePriorityGroup != "" {
		host := viper.GetString("orchestrator.host")
		priorityVal := viper.GetInt("orchestrator.priority_val")
		updateBulkPriority(host, updatePriorityMatch, updatePriorityTag, updatePriorityGroup, priorityVal)
		return nil
	}

	updateTimeoutJob := viper.GetString("orchestrator.update_timeout")
	updateTimeoutTag := viper.GetString("orchestrator.update_timeout_tag")
	updateTimeoutMatch := viper.GetString("orchestrator.update_timeout_match")

	timeoutFlagsSet := 0
	if updateTimeoutJob != "" {
		timeoutFlagsSet++
	}
	if updateTimeoutTag != "" {
		timeoutFlagsSet++
	}
	if updateTimeoutMatch != "" {
		timeoutFlagsSet++
	}

	if timeoutFlagsSet > 1 {
		fmt.Fprintf(stdout, "Error: Cannot use --update-timeout, --update-timeout-tag, and --update-timeout-match together. Please specify only one.\n")
		exitFunc(1)
		return nil
	}

	if updateTimeoutJob != "" {
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

	if updateTimeoutTag != "" || updateTimeoutMatch != "" {
		host := viper.GetString("orchestrator.host")
		timeoutVal := viper.GetString("orchestrator.timeout_val")
		if timeoutVal == "" {
			fmt.Fprintf(stdout, "Error: --timeout-val is required when using --update-timeout-tag or --update-timeout-match\n")
			exitFunc(1)
			return nil
		}
		updateBulkTimeout(host, updateTimeoutMatch, updateTimeoutTag, timeoutVal)
		return nil
	}

	updateMaxRetriesJob := viper.GetString("orchestrator.update_max_retries_job")
	updateMaxRetriesTag := viper.GetString("orchestrator.update_max_retries_tag")
	updateMaxRetriesMatch := viper.GetString("orchestrator.update_max_retries_match")

	maxRetriesFlagsSet := 0
	if updateMaxRetriesJob != "" {
		maxRetriesFlagsSet++
	}
	if updateMaxRetriesTag != "" {
		maxRetriesFlagsSet++
	}
	if updateMaxRetriesMatch != "" {
		maxRetriesFlagsSet++
	}

	if maxRetriesFlagsSet > 1 {
		fmt.Fprintf(stdout, "Error: Cannot use --update-max-retries-job, --update-max-retries-tag, and --update-max-retries-match together. Please specify only one.\n")
		exitFunc(1)
		return nil
	}

	if updateMaxRetriesJob != "" {
		host := viper.GetString("orchestrator.host")
		maxRetriesVal := viper.GetInt("orchestrator.max_retries_val")
		if !viper.IsSet("orchestrator.max_retries_val") || maxRetriesVal < 0 {
			fmt.Fprintf(stdout, "Error: valid --max-retries-val is required when using --update-max-retries-job\n")
			exitFunc(1)
			return nil
		}
		updateMaxRetries(host, updateMaxRetriesJob, maxRetriesVal)
		return nil
	}

	if updateMaxRetriesTag != "" || updateMaxRetriesMatch != "" {
		host := viper.GetString("orchestrator.host")
		maxRetriesVal := viper.GetInt("orchestrator.max_retries_val")
		if !viper.IsSet("orchestrator.max_retries_val") || maxRetriesVal < 0 {
			fmt.Fprintf(stdout, "Error: valid --max-retries-val is required when using --update-max-retries-tag or --update-max-retries-match\n")
			exitFunc(1)
			return nil
		}
		updateBulkMaxRetries(host, updateMaxRetriesMatch, updateMaxRetriesTag, maxRetriesVal)
		return nil
	}

	updateAgentJob := viper.GetString("orchestrator.update_agent_job")
	updateAgentTag := viper.GetString("orchestrator.update_agent_tag")
	updateAgentMatch := viper.GetString("orchestrator.update_agent_match")

	agentFlagsSet := 0
	if updateAgentJob != "" {
		agentFlagsSet++
	}
	if updateAgentTag != "" {
		agentFlagsSet++
	}
	if updateAgentMatch != "" {
		agentFlagsSet++
	}

	if agentFlagsSet > 1 {
		fmt.Fprintf(stdout, "Error: Cannot use --update-agent-job, --update-agent-tag, and --update-agent-match together. Please specify only one.\n")
		exitFunc(1)
		return nil
	}

	if updateAgentJob != "" {
		host := viper.GetString("orchestrator.host")
		providerVal := viper.GetString("orchestrator.agent_provider_val")
		modelVal := viper.GetString("orchestrator.agent_model_val")
		if providerVal == "" && modelVal == "" {
			fmt.Fprintf(stdout, "Error: --agent-provider-val or --agent-model-val is required when using --update-agent-job\n")
			exitFunc(1)
			return nil
		}
		updateAgent(host, updateAgentJob, providerVal, modelVal)
		return nil
	}

	if updateAgentTag != "" || updateAgentMatch != "" {
		host := viper.GetString("orchestrator.host")
		providerVal := viper.GetString("orchestrator.agent_provider_val")
		modelVal := viper.GetString("orchestrator.agent_model_val")
		if providerVal == "" && modelVal == "" {
			fmt.Fprintf(stdout, "Error: --agent-provider-val or --agent-model-val is required when using --update-agent-tag or --update-agent-match\n")
			exitFunc(1)
			return nil
		}
		updateBulkAgent(host, updateAgentMatch, updateAgentTag, providerVal, modelVal)
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

	updateDepsJob := viper.GetString("orchestrator.update_deps_job")
	updateDepsTag := viper.GetString("orchestrator.update_deps_tag")
	updateDepsMatch := viper.GetString("orchestrator.update_deps_match")

	depsFlagsSet := 0
	if updateDepsJob != "" {
		depsFlagsSet++
	}
	if updateDepsTag != "" {
		depsFlagsSet++
	}
	if updateDepsMatch != "" {
		depsFlagsSet++
	}

	if depsFlagsSet > 1 {
		fmt.Fprintf(stdout, "Error: Cannot use --update-deps-job, --update-deps-tag, and --update-deps-match together. Please specify only one.\n")
		exitFunc(1)
		return nil
	}

	if updateDepsJob != "" {
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

	if updateDepsTag != "" || updateDepsMatch != "" {
		host := viper.GetString("orchestrator.host")
		var setDepsPtr []string
		if viper.IsSet("orchestrator.set_deps") {
			setDepsPtr = viper.GetStringSlice("orchestrator.set_deps")
		} else {
			setDepsPtr = []string{}
		}
		updateBulkDependencies(host, updateDepsMatch, updateDepsTag, setDepsPtr)
		return nil
	}

	updateEnvJob := viper.GetString("orchestrator.update_env_job")
	updateEnvTag := viper.GetString("orchestrator.update_env_tag")
	updateEnvMatch := viper.GetString("orchestrator.update_env_match")

	envFlagsSet := 0
	if updateEnvJob != "" {
		envFlagsSet++
	}
	if updateEnvTag != "" {
		envFlagsSet++
	}
	if updateEnvMatch != "" {
		envFlagsSet++
	}

	if envFlagsSet > 1 {
		fmt.Fprintf(stdout, "Error: Cannot use --update-env-job, --update-env-tag, and --update-env-match together. Please specify only one.\n")
		exitFunc(1)
		return nil
	}

	if updateEnvJob != "" {
		host := viper.GetString("orchestrator.host")
		var setEnvSlice []string
		if viper.IsSet("orchestrator.set_env") {
			setEnvSlice = viper.GetStringSlice("orchestrator.set_env")
		} else {
			setEnvSlice = []string{}
		}

		envMap := make(map[string]string)
		for _, pair := range setEnvSlice {
			parts := strings.SplitN(pair, "=", 2)
			if len(parts) == 2 {
				envMap[parts[0]] = parts[1]
			} else {
				logger.Warn("Invalid environment variable format", "input", pair)
			}
		}

		updateEnvVars(host, updateEnvJob, envMap)
		return nil
	}

	if updateEnvTag != "" || updateEnvMatch != "" {
		host := viper.GetString("orchestrator.host")
		var setEnvSlice []string
		if viper.IsSet("orchestrator.set_env") {
			setEnvSlice = viper.GetStringSlice("orchestrator.set_env")
		} else {
			setEnvSlice = []string{}
		}

		envMap := make(map[string]string)
		for _, pair := range setEnvSlice {
			parts := strings.SplitN(pair, "=", 2)
			if len(parts) == 2 {
				envMap[parts[0]] = parts[1]
			} else {
				logger.Warn("Invalid environment variable format", "input", pair)
			}
		}

		updateBulkEnvVars(host, updateEnvMatch, updateEnvTag, envMap)
		return nil
	}

	updateTagsJob := viper.GetString("orchestrator.update_tags_job")
	updateTagsTag := viper.GetString("orchestrator.update_tags_tag")
	updateTagsMatch := viper.GetString("orchestrator.update_tags_match")

	tagsFlagsSet := 0
	if updateTagsJob != "" {
		tagsFlagsSet++
	}
	if updateTagsTag != "" {
		tagsFlagsSet++
	}
	if updateTagsMatch != "" {
		tagsFlagsSet++
	}

	if tagsFlagsSet > 1 {
		fmt.Fprintf(stdout, "Error: Cannot use --update-tags-job, --update-tags-tag, and --update-tags-match together. Please specify only one.\n")
		exitFunc(1)
		return nil
	}

	if updateTagsJob != "" {
		host := viper.GetString("orchestrator.host")
		var setTagsPtr []string
		if viper.IsSet("orchestrator.set_tags") {
			setTagsPtr = viper.GetStringSlice("orchestrator.set_tags")
		} else {
			setTagsPtr = []string{}
		}
		updateTags(host, updateTagsJob, setTagsPtr)
		return nil
	}

	if updateTagsTag != "" || updateTagsMatch != "" {
		host := viper.GetString("orchestrator.host")
		var setTagsPtr []string
		if viper.IsSet("orchestrator.set_tags") {
			setTagsPtr = viper.GetStringSlice("orchestrator.set_tags")
		} else {
			setTagsPtr = []string{}
		}
		updateBulkTags(host, updateTagsMatch, updateTagsTag, setTagsPtr)
		return nil
	}

	addTagJob := viper.GetString("orchestrator.add_tag_job")
	addTagTag := viper.GetString("orchestrator.add_tag_tag")
	addTagMatch := viper.GetString("orchestrator.add_tag_match")

	addTagsFlagsSet := 0
	if addTagJob != "" {
		addTagsFlagsSet++
	}
	if addTagTag != "" {
		addTagsFlagsSet++
	}
	if addTagMatch != "" {
		addTagsFlagsSet++
	}

	if addTagsFlagsSet > 1 {
		fmt.Fprintf(stdout, "Error: Cannot use --add-tag-job, --add-tag-tag, and --add-tag-match together. Please specify only one.\n")
		exitFunc(1)
		return nil
	}

	if addTagJob != "" {
		host := viper.GetString("orchestrator.host")
		var setTagsPtr []string
		if viper.IsSet("orchestrator.set_tags") {
			setTagsPtr = viper.GetStringSlice("orchestrator.set_tags")
		} else {
			setTagsPtr = []string{}
		}
		addTags(host, addTagJob, setTagsPtr)
		return nil
	}

	if addTagTag != "" || addTagMatch != "" {
		host := viper.GetString("orchestrator.host")
		var setTagsPtr []string
		if viper.IsSet("orchestrator.set_tags") {
			setTagsPtr = viper.GetStringSlice("orchestrator.set_tags")
		} else {
			setTagsPtr = []string{}
		}
		addBulkTags(host, addTagMatch, addTagTag, setTagsPtr)
		return nil
	}

	removeTagJob := viper.GetString("orchestrator.remove_tag_job")
	removeTagTag := viper.GetString("orchestrator.remove_tag_tag")
	removeTagMatch := viper.GetString("orchestrator.remove_tag_match")

	removeTagsFlagsSet := 0
	if removeTagJob != "" {
		removeTagsFlagsSet++
	}
	if removeTagTag != "" {
		removeTagsFlagsSet++
	}
	if removeTagMatch != "" {
		removeTagsFlagsSet++
	}

	if removeTagsFlagsSet > 1 {
		fmt.Fprintf(stdout, "Error: Cannot use --remove-tag-job, --remove-tag-tag, and --remove-tag-match together. Please specify only one.\n")
		exitFunc(1)
		return nil
	}

	if removeTagJob != "" {
		host := viper.GetString("orchestrator.host")
		var setTagsPtr []string
		if viper.IsSet("orchestrator.set_tags") {
			setTagsPtr = viper.GetStringSlice("orchestrator.set_tags")
		} else {
			setTagsPtr = []string{}
		}
		removeTags(host, removeTagJob, setTagsPtr)
		return nil
	}

	if removeTagTag != "" || removeTagMatch != "" {
		host := viper.GetString("orchestrator.host")
		var setTagsPtr []string
		if viper.IsSet("orchestrator.set_tags") {
			setTagsPtr = viper.GetStringSlice("orchestrator.set_tags")
		} else {
			setTagsPtr = []string{}
		}
		removeBulkTags(host, removeTagMatch, removeTagTag, setTagsPtr)
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

	if waitMatch := viper.GetString("orchestrator.wait_match"); waitMatch != "" {
		host := viper.GetString("orchestrator.host")
		if err := waitForMatch(host, waitMatch, stdout); err != nil {
			fmt.Fprintf(stdout, "Match %s wait failed: %v\n", waitMatch, err)
			exitFunc(1)
		}
		return nil
	}

	if waitGroup := viper.GetString("orchestrator.wait_group"); waitGroup != "" {
		host := viper.GetString("orchestrator.host")
		if err := waitForGroup(host, waitGroup, stdout); err != nil {
			fmt.Fprintf(stdout, "Group %s wait failed: %v\n", waitGroup, err)
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

	if validatePipelineFile := viper.GetString("orchestrator.validate_pipeline"); validatePipelineFile != "" {
		target := viper.GetString("orchestrator.submit_pipeline_target")
		varsList := viper.GetStringSlice("orchestrator.pipeline_var")
		varFile := viper.GetString("orchestrator.pipeline_var_file")
		vars, err := loadPipelineVars(varsList, varFile)
		if err != nil {
			fmt.Fprintf(stdout, "Failed to load variables: %v\n", err)
			exitFunc(1)
			return nil
		}
		validatePipeline(validatePipelineFile, target, vars)
		return nil
	}

	if submitPipelineFile := viper.GetString("orchestrator.submit_pipeline"); submitPipelineFile != "" {
		host := viper.GetString("orchestrator.host")
		wait := viper.GetBool("orchestrator.wait")
		dryRun := viper.GetBool("orchestrator.dry_run_pipeline")
		target := viper.GetString("orchestrator.submit_pipeline_target")
		interactive := viper.GetBool("orchestrator.submit_pipeline_interactive")

		varsList := viper.GetStringSlice("orchestrator.pipeline_var")
		varFile := viper.GetString("orchestrator.pipeline_var_file")
		vars, err := loadPipelineVars(varsList, varFile)
		if err != nil {
			fmt.Fprintf(stdout, "Failed to load variables: %v\n", err)
			exitFunc(1)
			return nil
		}

		if interactive {
			submitPipelineInteractiveJob(host, submitPipelineFile, wait, dryRun, target, vars)
		} else {
			submitPipelineJob(host, submitPipelineFile, wait, dryRun, target, vars)
		}
		return nil
	}

	if lintPipelineFile := viper.GetString("orchestrator.lint_pipeline"); lintPipelineFile != "" {
		target := viper.GetString("orchestrator.submit_pipeline_target")

		varsList := viper.GetStringSlice("orchestrator.pipeline_var")
		varFile := viper.GetString("orchestrator.pipeline_var_file")
		vars, err := loadPipelineVars(varsList, varFile)
		if err != nil {
			fmt.Fprintf(stdout, "Failed to load variables: %v\n", err)
			exitFunc(1)
			return nil
		}

		lintPipelineJob(lintPipelineFile, target, vars)
		return nil
	}

	if importPipelineFile := viper.GetString("orchestrator.import_pipeline"); importPipelineFile != "" {
		host := viper.GetString("orchestrator.host")
		target := viper.GetString("orchestrator.submit_pipeline_target")

		varsList := viper.GetStringSlice("orchestrator.pipeline_var")
		varFile := viper.GetString("orchestrator.pipeline_var_file")
		vars, err := loadPipelineVars(varsList, varFile)
		if err != nil {
			fmt.Fprintf(stdout, "Failed to load variables: %v\n", err)
			exitFunc(1)
			return nil
		}

		importPipelineJob(host, importPipelineFile, target, vars)
		return nil
	}

	if explainPipelineFile := viper.GetString("orchestrator.explain_pipeline"); explainPipelineFile != "" {
		target := viper.GetString("orchestrator.submit_pipeline_target")

		varsList := viper.GetStringSlice("orchestrator.pipeline_var")
		varFile := viper.GetString("orchestrator.pipeline_var_file")
		vars, err := loadPipelineVars(varsList, varFile)
		if err != nil {
			fmt.Fprintf(stdout, "Failed to load variables: %v\n", err)
			exitFunc(1)
			return nil
		}

		explainPipelineJob(explainPipelineFile, target, vars)
		return nil
	}

	if exportPipelineGraphFile := viper.GetString("orchestrator.export_pipeline_graph"); exportPipelineGraphFile != "" {
		target := viper.GetString("orchestrator.submit_pipeline_target")
		format := viper.GetString("orchestrator.export_pipeline_graph_format")
		outFile := viper.GetString("orchestrator.export_pipeline_graph_out")

		varsList := viper.GetStringSlice("orchestrator.pipeline_var")
		varFile := viper.GetString("orchestrator.pipeline_var_file")
		vars, err := loadPipelineVars(varsList, varFile)
		if err != nil {
			fmt.Fprintf(stdout, "Failed to load variables: %v\n", err)
			exitFunc(1)
			return nil
		}

		exportPipelineGraphJob(exportPipelineGraphFile, target, vars, format, outFile)
		return nil
	}

	if listTemplatesFile := viper.GetString("orchestrator.list_templates"); listTemplatesFile != "" {
		varsList := viper.GetStringSlice("orchestrator.pipeline_var")
		varFile := viper.GetString("orchestrator.pipeline_var_file")
		vars, err := loadPipelineVars(varsList, varFile)
		if err != nil {
			fmt.Fprintf(stdout, "Failed to load variables: %v\n", err)
			exitFunc(1)
			return nil
		}

		listTemplatesJob(listTemplatesFile, vars)
		return nil
	}

	if listPipelineVarsFile := viper.GetString("orchestrator.list_pipeline_vars"); listPipelineVarsFile != "" {
		format := viper.GetString("orchestrator.format")
		listPipelineVarsJob(listPipelineVarsFile, format)
		return nil
	}

	if inspectPipelineFile := viper.GetString("orchestrator.inspect_pipeline"); inspectPipelineFile != "" {
		target := viper.GetString("orchestrator.submit_pipeline_target")

		varsList := viper.GetStringSlice("orchestrator.pipeline_var")
		varFile := viper.GetString("orchestrator.pipeline_var_file")
		vars, err := loadPipelineVars(varsList, varFile)
		if err != nil {
			fmt.Fprintf(stdout, "Failed to load variables: %v\n", err)
			exitFunc(1)
			return nil
		}

		inspectPipelineJob(inspectPipelineFile, target, vars)
		return nil
	}

	if comparePipelinesStr := viper.GetString("orchestrator.compare_pipelines"); comparePipelinesStr != "" {
		comparePipelines(comparePipelinesStr)
		return nil
	}

	if applyPipelineFile := viper.GetString("orchestrator.apply_pipeline"); applyPipelineFile != "" {
		host := viper.GetString("orchestrator.host")
		dryRun := viper.GetBool("orchestrator.apply_pipeline_dry_run")
		target := viper.GetString("orchestrator.submit_pipeline_target")
		runID := viper.GetString("orchestrator.apply_pipeline_run_id")

		varsList := viper.GetStringSlice("orchestrator.pipeline_var")
		varFile := viper.GetString("orchestrator.pipeline_var_file")
		vars, err := loadPipelineVars(varsList, varFile)
		if err != nil {
			fmt.Fprintf(stdout, "Failed to load variables: %v\n", err)
			exitFunc(1)
			return nil
		}

		applyPipelineJob(host, applyPipelineFile, dryRun, target, vars, runID)
		return nil
	}

	if watchPipelineFile := viper.GetString("orchestrator.watch_pipeline"); watchPipelineFile != "" {
		host := viper.GetString("orchestrator.host")
		dryRun := viper.GetBool("orchestrator.apply_pipeline_dry_run")
		target := viper.GetString("orchestrator.submit_pipeline_target")
		runID := viper.GetString("orchestrator.apply_pipeline_run_id")
		interval := viper.GetDuration("orchestrator.watch_pipeline_interval")

		varsList := viper.GetStringSlice("orchestrator.pipeline_var")
		varFile := viper.GetString("orchestrator.pipeline_var_file")
		vars, err := loadPipelineVars(varsList, varFile)
		if err != nil {
			fmt.Fprintf(stdout, "Failed to load variables: %v\n", err)
			exitFunc(1)
			return nil
		}

		watchPipelineJob(ctx, host, watchPipelineFile, dryRun, target, vars, runID, interval)
		return nil
	}

	if query := viper.GetString("orchestrator.search_logs"); query != "" {
		host := viper.GetString("orchestrator.host")
		tag := viper.GetString("orchestrator.search_tag")
		status := viper.GetString("orchestrator.search_status")
		contextLines := viper.GetInt("orchestrator.search_context")
		searchLogs(host, query, tag, status, contextLines)
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

		var dependencyTimeoutPtr *time.Duration
		if viper.IsSet("orchestrator.submit_dependency_timeout") {
			dt := viper.GetDuration("orchestrator.submit_dependency_timeout")
			if dt > 0 {
				dependencyTimeoutPtr = &dt
			}
		}

		var maxRetriesPtr *int
		if viper.IsSet("orchestrator.submit_max_retries") {
			mr := viper.GetInt("orchestrator.submit_max_retries")
			if mr >= 0 {
				maxRetriesPtr = &mr
			}
		}

		concurrencyGroup := viper.GetString("orchestrator.submit_concurrency_group")
		cancelInProgress := viper.GetBool("orchestrator.submit_cancel_in_progress")
		agentProvider := viper.GetString("orchestrator.submit_agent_provider")
		agentModel := viper.GetString("orchestrator.submit_agent_model")
		var requireApprovalPtr *bool
		if viper.IsSet("orchestrator.submit_require_approval") {
			ra := viper.GetBool("orchestrator.submit_require_approval")
			requireApprovalPtr = &ra
		}

		var retryDelayPtr *time.Duration
		if viper.IsSet("orchestrator.submit_retry_delay") {
			rd := viper.GetDuration("orchestrator.submit_retry_delay")
			retryDelayPtr = &rd
		}

		var retryBackoffPtr *float64
		if viper.IsSet("orchestrator.submit_retry_backoff") {
			rb := viper.GetFloat64("orchestrator.submit_retry_backoff")
			retryBackoffPtr = &rb
		}

		runCondition := viper.GetString("orchestrator.submit_run_condition")
		webhookURL := viper.GetString("orchestrator.submit_webhook_url")
		autoHeal := viper.GetBool("orchestrator.submit_auto_heal")

		// Note: pflag.StringArray is correctly captured as a string slice by viper.GetStringSlice,
		// but since viper.GetStringSlice might internally split by comma if parsing environment variables,
		// we must be careful. Let's use the raw pflag value if we can to avoid unexpected behavior, or rely on viper's handling
		// of the underlying `StringArray` which *usually* doesn't split by comma (unlike `StringSlice`).
		// Actually, viper.GetStringSlice on a `StringArray` flag returns the array verbatim.
		if matrixInline := viper.GetStringSlice("orchestrator.submit_matrix_inline"); len(matrixInline) > 0 {
			matrixMap := make(map[string][]string)
			for _, item := range matrixInline {
				parts := strings.SplitN(item, "=", 2)
				if len(parts) == 2 {
					vals := strings.Split(parts[1], ",")
					for i := range vals {
						vals[i] = strings.TrimSpace(vals[i])
					}
					matrixMap[strings.TrimSpace(parts[0])] = vals
				} else {
					logger.Warn("Invalid inline matrix format", "input", item)
				}
			}
			submitMatrixInlineJob(host, submitURL, task, id, priority, delay, timeout, dependencyTimeoutPtr, maxRetriesPtr, requireApprovalPtr, retryDelayPtr, retryBackoffPtr, wait, envMap, submitDeps, submitTags, concurrencyGroup, cancelInProgress, agentProvider, agentModel, runCondition, webhookURL, autoHeal, matrixMap)
			return nil
		}

		submitAdHocJob(host, submitURL, task, id, priority, delay, timeout, dependencyTimeoutPtr, maxRetriesPtr, requireApprovalPtr, retryDelayPtr, retryBackoffPtr, wait, envMap, submitDeps, submitTags, concurrencyGroup, cancelInProgress, agentProvider, agentModel, runCondition, webhookURL, autoHeal)
		return nil
	}

	if exportSingleFile := viper.GetString("orchestrator.export_job"); exportSingleFile != "" {
		host := viper.GetString("orchestrator.host")
		outPath := viper.GetString("orchestrator.export_job_out")
		exportSingleJob(host, exportSingleFile, outPath)
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
	if exportPipelineFile := viper.GetString("orchestrator.export_pipeline"); exportPipelineFile != "" {
		host := viper.GetString("orchestrator.host")
		exportPipeline(host, exportPipelineFile)
		return nil
	}
	if exportGraphFile := viper.GetString("orchestrator.export_graph"); exportGraphFile != "" {
		host := viper.GetString("orchestrator.host")
		format := viper.GetString("orchestrator.export_graph_format")
		exportGraph(host, exportGraphFile, format)
		return nil
	}
	if exportMetricsFile := viper.GetString("orchestrator.export_metrics"); exportMetricsFile != "" {
		host := viper.GetString("orchestrator.host")
		state := viper.GetString("orchestrator.export_metrics_state")
		exportMetrics(host, exportMetricsFile, state)
		return nil
	}

	if exportJunitFile := viper.GetString("orchestrator.export_junit"); exportJunitFile != "" {
		host := viper.GetString("orchestrator.host")
		exportJunit(host, exportJunitFile)
		return nil
	}
	if exportTraceFile := viper.GetString("orchestrator.export_trace"); exportTraceFile != "" {
		host := viper.GetString("orchestrator.host")
		state := viper.GetString("orchestrator.export_trace_state")
		exportTrace(host, exportTraceFile, state)
		return nil
	}
	if exportTimelineFile := viper.GetString("orchestrator.export_timeline"); exportTimelineFile != "" {
		host := viper.GetString("orchestrator.host")
		state := viper.GetString("orchestrator.export_timeline_state")
		exportTimeline(host, exportTimelineFile, state)
		return nil
	}
	if exportCostsFile := viper.GetString("orchestrator.export_costs"); exportCostsFile != "" {
		host := viper.GetString("orchestrator.host")
		format := viper.GetString("orchestrator.export_costs_format")
		exportCosts(host, exportCostsFile, format)
		return nil
	}
	if exportAgentsFile := viper.GetString("orchestrator.export_agents"); exportAgentsFile != "" {
		host := viper.GetString("orchestrator.host")
		format := viper.GetString("orchestrator.export_agents_format")
		exportAgents(host, exportAgentsFile, format)
		return nil
	}
	if exportTagsFile := viper.GetString("orchestrator.export_tags"); exportTagsFile != "" {
		host := viper.GetString("orchestrator.host")
		format := viper.GetString("orchestrator.export_tags_format")
		exportTags(host, exportTagsFile, format, viper.GetInt("orchestrator.analyze_tags_limit"))
		return nil
	}

	if exportAnomaliesFile := viper.GetString("orchestrator.export_anomalies"); exportAnomaliesFile != "" {
		host := viper.GetString("orchestrator.host")
		format := viper.GetString("orchestrator.export_anomalies_format")
		exportAnomalies(host, exportAnomaliesFile, format)
		return nil
	}

	if uploadFilePath := viper.GetString("orchestrator.upload_artifact"); uploadFilePath != "" {
		host := viper.GetString("orchestrator.host")
		jobID := viper.GetString("orchestrator.job_id")
		if jobID == "" {
			fmt.Fprintf(stdout, "Error: --job-id is required when using --upload-artifact\n")
			exitFunc(1)
			return nil
		}
		uploadArtifact(host, jobID, uploadFilePath)
		return nil
	}

	if downloadFilename := viper.GetString("orchestrator.download_artifact"); downloadFilename != "" {
		host := viper.GetString("orchestrator.host")
		jobID := viper.GetString("orchestrator.job_id")
		if jobID == "" {
			fmt.Fprintf(stdout, "Error: --job-id is required when using --download-artifact\n")
			exitFunc(1)
			return nil
		}
		outPath := viper.GetString("orchestrator.artifact_out")
		downloadArtifact(host, jobID, downloadFilename, outPath)
		return nil
	}

	if viper.GetBool("orchestrator.list_artifacts") {
		host := viper.GetString("orchestrator.host")
		jobID := viper.GetString("orchestrator.job_id")
		if jobID == "" {
			fmt.Fprintf(stdout, "Error: --job-id is required when using --list-artifacts\n")
			exitFunc(1)
			return nil
		}
		listArtifacts(host, jobID)
		return nil
	}

	if deleteFilename := viper.GetString("orchestrator.delete_artifact"); deleteFilename != "" {
		host := viper.GetString("orchestrator.host")
		jobID := viper.GetString("orchestrator.job_id")
		if jobID == "" {
			fmt.Fprintf(stdout, "Error: --job-id is required when using --delete-artifact\n")
			exitFunc(1)
			return nil
		}
		deleteArtifact(host, jobID, deleteFilename)
		return nil
	}

	if generatePrompt := viper.GetString("orchestrator.generate_pipeline"); generatePrompt != "" {
		host := viper.GetString("orchestrator.host")
		outFile := viper.GetString("orchestrator.generate_pipeline_out")
		provider := viper.GetString("orchestrator.agent_provider")
		model := viper.GetString("orchestrator.agent_model")
		generatePipeline(host, generatePrompt, outFile, provider, model)
		return nil
	}

	if outFile := viper.GetString("orchestrator.generate_changelog"); outFile != "" {
		host := viper.GetString("orchestrator.host")
		tag := viper.GetString("orchestrator.changelog_tag")
		match := viper.GetString("orchestrator.changelog_match")
		provider := viper.GetString("orchestrator.agent_provider")
		model := viper.GetString("orchestrator.agent_model")
		generateChangelog(host, outFile, tag, match, provider, model)
		return nil
	}

	if outFile := viper.GetString("orchestrator.generate_postmortem"); outFile != "" {
		host := viper.GetString("orchestrator.host")
		tag := viper.GetString("orchestrator.postmortem_tag")
		match := viper.GetString("orchestrator.postmortem_match")
		provider := viper.GetString("orchestrator.agent_provider")
		model := viper.GetString("orchestrator.agent_model")
		generatePostmortem(host, outFile, tag, match, provider, model)
		return nil
	}

	if optimizeFile := viper.GetString("orchestrator.optimize_pipeline"); optimizeFile != "" {
		outFile := viper.GetString("orchestrator.optimize_pipeline_out")
		provider := viper.GetString("orchestrator.agent_provider")
		model := viper.GetString("orchestrator.agent_model")
		optimizePipelineJob(optimizeFile, outFile, provider, model)
		return nil
	}

	if viper.GetBool("orchestrator.monitor") {
		host := viper.GetString("orchestrator.host")
		if err := tui.StartDashboard(host); err != nil {
			return fmt.Errorf("Dashboard failed: %w", err)
		}
		return nil
	}

	if viper.GetBool("orchestrator.stream_events") {
		host := viper.GetString("orchestrator.host")
		if err := streamEvents(ctx, host); err != nil {
			fmt.Fprintf(stdout, "Stream events failed: %v\n", err)
			exitFunc(1)
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

	allowedPollers := viper.GetStringSlice("orchestrator.allowed_pollers")
	if len(allowedPollers) > 0 {
		isAllowed := false
		requestedPoller := pollerType
		if requestedPoller == "" {
			requestedPoller = "jira"
		}
		for _, ap := range allowedPollers {
			if ap == requestedPoller {
				isAllowed = true
				break
			}
		}
		if !isAllowed {
			return fmt.Errorf("poller '%s' is not in the allowed pollers list: %v", requestedPoller, allowedPollers)
		}
	}

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
		spawner = orchestrator.NewProcessSpawner(logger, poller, agentProvider, agentModel, sm, maxIterations, managerFrequency, taskMaxIterations)
	default:
		return fmt.Errorf("Invalid mode. Use 'local', 'k8s', or 'process': %s", mode)
	}

	// 3. Janitor
	logDir := viper.GetString("orchestrator.log_dir")
	if viper.GetBool("orchestrator.cleanup") && (janitorClient != nil || logDir != "") {
		janitor := orchestrator.NewJanitor(
			logger,
			janitorClient, // Can be nil
			viper.GetDuration("orchestrator.cleanup_interval"),
			viper.GetDuration("orchestrator.cleanup_age"),
			viper.GetBool("orchestrator.cleanup_dry_run"),
			logDir,
		)
		go janitor.Start(ctx)
	} else if viper.GetBool("orchestrator.cleanup") {
		logger.Warn("Cleanup enabled but neither docker client nor log_dir are configured")
	}

	// 4. Orchestrator
	orch := orchestrator.New(poller, spawner, interval)
	orch.MaxConcurrentJobs = viper.GetInt("orchestrator.max_concurrent_jobs")
	orch.JobTimeout = viper.GetDuration("orchestrator.job_timeout")
	orch.MaxRetries = viper.GetInt("orchestrator.max_retries")
	orch.MaxBudget = viper.GetFloat64("orchestrator.max_budget")
	orch.LogDir = viper.GetString("orchestrator.log_dir")
	orch.ArtifactsDir = viper.GetString("orchestrator.artifacts_dir")
	orch.RetryDelay = viper.GetDuration("orchestrator.retry_delay")
	orch.CircuitBreakerMaxFailures = viper.GetInt("orchestrator.circuit_breaker_max")
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

func printAnalytics(host, format string) {
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

	if format == "json" {
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(analytics); err != nil {
			fmt.Fprintf(stdout, "Failed to encode analytics to JSON: %v\n", err)
			exitFunc(1)
		}
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

func printStatus(host, format string) {
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

	if format == "json" {
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(status); err != nil {
			fmt.Fprintf(stdout, "Failed to encode status to JSON: %v\n", err)
			exitFunc(1)
		}
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
	printField("Draining", fmt.Sprintf("%t", status.Draining))
	printField("Circuit Broken", fmt.Sprintf("%t", status.CircuitBroken))
	if status.MaxConcurrentJobs > 0 {
		printField("Max Concurrent Jobs", fmt.Sprintf("%d", status.MaxConcurrentJobs))
	} else {
		printField("Max Concurrent Jobs", "Unlimited")
	}
	fmt.Fprintln(stdout, "")
}

func listPendingJobs(host string, priority string, format string) {
	u, err := url.Parse(fmt.Sprintf("%s/jobs", host))
	if err != nil {
		fmt.Fprintf(stdout, "Failed to parse host URL: %v\n", err)
		exitFunc(1)
		return
	}

	q := u.Query()
	q.Set("state", "pending")
	if priority != "" {
		q.Set("priority", priority)
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

	if format == "csv" {
		writer := csv.NewWriter(stdout)
		defer writer.Flush()

		writer.Write([]string{"ID", "Summary", "Status", "Priority", "Tags"})
		for _, job := range jobs {
			writer.Write([]string{
				job.ID,
				job.Summary,
				job.Status,
				fmt.Sprintf("%d", job.WorkItem.Priority),
				strings.Join(job.WorkItem.Tags, ","),
			})
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

	// Define column widths explicitly using lipgloss Width
	idCol := lipgloss.NewStyle().Width(15)
	summaryCol := lipgloss.NewStyle().Width(40)
	statusCol := lipgloss.NewStyle().Width(25)
	durationCol := lipgloss.NewStyle().Width(20)

	// Table Header (using lipgloss width instead of fmt %-15s)
	fmt.Fprintf(stdout, "%s %s %s %s\n",
		idCol.Render(headerStyle.Render("ID")),
		summaryCol.Render(headerStyle.Render("Summary")),
		statusCol.Render(headerStyle.Render("Status")),
		durationCol.Render(headerStyle.Render("Duration")),
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

		fmt.Fprintf(stdout, "%s %s %s %s\n",
			idCol.Render(rowStyle.Render(job.ID)),
			summaryCol.Render(rowStyle.Render(limitString(job.Summary, 38))),
			statusCol.Render(rowStyle.Render(statusDisplay)),
			durationCol.Render(rowStyle.Render(duration)),
		)
	}
}

func listJobs(host string, history bool, status, tag, match, priority, format string) {
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
	if priority != "" {
		q.Set("priority", priority)
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

	if format == "csv" {
		writer := csv.NewWriter(stdout)
		defer writer.Flush()

		writer.Write([]string{"ID", "Summary", "Status", "Priority", "Tags", "Duration"})
		for _, job := range jobs {
			duration := time.Since(job.StartTime).Round(time.Second).String()
			writer.Write([]string{
				job.ID,
				job.Summary,
				job.Status,
				fmt.Sprintf("%d", job.WorkItem.Priority),
				strings.Join(job.WorkItem.Tags, ","),
				duration,
			})
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

	// Define column widths explicitly using lipgloss Width
	idCol := lipgloss.NewStyle().Width(15)
	summaryCol := lipgloss.NewStyle().Width(40)
	statusCol := lipgloss.NewStyle().Width(25)
	durationCol := lipgloss.NewStyle().Width(20)

	// Table Header (using lipgloss width instead of fmt %-15s)
	fmt.Fprintf(stdout, "%s %s %s %s\n",
		idCol.Render(headerStyle.Render("ID")),
		summaryCol.Render(headerStyle.Render("Summary")),
		statusCol.Render(headerStyle.Render("Status")),
		durationCol.Render(headerStyle.Render("Duration")),
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

		fmt.Fprintf(stdout, "%s %s %s %s\n",
			idCol.Render(rowStyle.Render(job.ID)),
			summaryCol.Render(rowStyle.Render(limitString(job.Summary, 38))),
			statusCol.Render(rowStyle.Render(statusDisplay)),
			durationCol.Render(rowStyle.Render(duration)),
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
			if utils.ContainsFold(k, "token") || utils.ContainsFold(k, "key") || utils.ContainsFold(k, "secret") {
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

	// Blockers
	if len(job.WorkItem.DependsOn) > 0 && (job.Status == "Pending" || job.Status == "Pending Approval") {
		blockersResp, err := http.Get(fmt.Sprintf("%s/jobs/%s/blockers", host, jobID))
		if err == nil && blockersResp.StatusCode == http.StatusOK {
			var blockers []orchestrator.JobInfo
			if err := json.NewDecoder(blockersResp.Body).Decode(&blockers); err == nil && len(blockers) > 0 {
				fmt.Fprintln(stdout, "\n"+labelStyle.Render("Blockers:"))
				for _, blocker := range blockers {
					statusStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
					if blocker.Status == "Failed" || blocker.Status == "Missing" || blocker.Status == "Canceled" {
						statusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
					} else if blocker.Status == "Running" || blocker.Status == "Active" || blocker.Status == "Spawning" {
						statusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
					}
					fmt.Fprintf(stdout, "  - %s (%s)\n", blocker.ID, statusStyle.Render(blocker.Status))
				}
			}
			blockersResp.Body.Close()
		}
	}

	// Dependents
	dependentsResp, err := http.Get(fmt.Sprintf("%s/jobs/%s/dependents", host, jobID))
	if err == nil && dependentsResp.StatusCode == http.StatusOK {
		var dependents []orchestrator.JobInfo
		if err := json.NewDecoder(dependentsResp.Body).Decode(&dependents); err == nil && len(dependents) > 0 {
			fmt.Fprintln(stdout, "\n"+labelStyle.Render("Dependents:"))
			for _, dependent := range dependents {
				statusStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
				if dependent.Status == "Failed" || dependent.Status == "Missing" || dependent.Status == "Canceled" {
					statusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
				} else if dependent.Status == "Running" || dependent.Status == "Active" || dependent.Status == "Spawning" {
					statusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
				} else if dependent.Status == "Completed" || dependent.Status == "Skipped" {
					statusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("46"))
				}
				fmt.Fprintf(stdout, "  - %s (%s)\n", dependent.ID, statusStyle.Render(dependent.Status))
			}
		}
		dependentsResp.Body.Close()
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

func cancelJob(host, jobID string, downstream bool) {
	url := fmt.Sprintf("%s/jobs/%s", host, jobID)
	if downstream {
		url += "?downstream=true"
	}

	req, err := http.NewRequest(http.MethodDelete, url, nil)
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

	if downstream {
		var result struct {
			CanceledJobs []string `json:"canceled_jobs"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			fmt.Fprintf(stdout, "Failed to decode response: %v\n", err)
			exitFunc(1)
			return
		}
		fmt.Fprintf(stdout, "Job %s and its downstream dependencies cancelled successfully.\n", jobID)
		if len(result.CanceledJobs) > 0 {
			fmt.Fprintf(stdout, "Canceled jobs: %s\n", strings.Join(result.CanceledJobs, ", "))
		}
	} else {
		fmt.Fprintf(stdout, "Job %s cancelled successfully.\n", jobID)
	}
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

func updatePollInterval(host, interval string) {
	reqBody := fmt.Sprintf(`{"interval": "%s"}`, interval)
	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/interval", host), strings.NewReader(reqBody))
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
		fmt.Fprintf(stdout, "Failed to update poll interval: %s\n", strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	fmt.Fprintf(stdout, "Orchestrator poll interval updated to %s.\n", interval)
}

func retryJob(host, jobID string, downstream bool, envVars map[string]string, provider, model string) {
	url := fmt.Sprintf("%s/jobs/%s/retry", host, jobID)
	if downstream {
		url += "?downstream=true"
	}

	reqBody := struct {
		EnvVars       map[string]string `json:"env_vars,omitempty"`
		AgentProvider string            `json:"agent_provider,omitempty"`
		AgentModel    string            `json:"agent_model,omitempty"`
	}{
		EnvVars:       envVars,
		AgentProvider: provider,
		AgentModel:    model,
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to marshal overrides: %v\n", err)
		exitFunc(1)
		return
	}

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(payload))
	if err != nil {
		fmt.Fprintf(stdout, "Failed to create request: %v\n", err)
		exitFunc(1)
		return
	}
	req.Header.Set("Content-Type", "application/json")

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

	if downstream {
		var result struct {
			RetriedJobs []string `json:"retried_jobs"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			fmt.Fprintf(stdout, "Failed to decode response: %v\n", err)
			exitFunc(1)
			return
		}
		fmt.Fprintf(stdout, "Job %s and its downstream dependencies retried successfully.\n", jobID)
		if len(result.RetriedJobs) > 0 {
			fmt.Fprintf(stdout, "Retried jobs: %s\n", strings.Join(result.RetriedJobs, ", "))
		}
	} else {
		fmt.Fprintf(stdout, "Job %s retry submitted successfully.\n", jobID)
	}
}

func retryFailedJobs(host, match, tag, group string) {
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
	if group != "" {
		q.Set("group", group)
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

func approveBulkJobs(host, match, tag, group, olderThan string) {
	u, err := url.Parse(fmt.Sprintf("%s/jobs/approve", host))
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
	if group != "" {
		q.Set("group", group)
	}
	if olderThan != "" {
		q.Set("older_than", olderThan)
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
		fmt.Fprintf(stdout, "Failed to approve jobs: %s\n", strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	var result struct {
		Approved int `json:"approved"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		fmt.Fprintf(stdout, "Failed to decode response: %v\n", err)
		exitFunc(1)
		return
	}

	fmt.Fprintf(stdout, "Successfully approved %d jobs.\n", result.Approved)
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
func deletePendingJob(host, jobID string) {
	req, err := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/jobs/%s/pending", host, jobID), nil)
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
		fmt.Fprintf(stdout, "Failed to delete pending job: %s\n", strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	fmt.Fprintf(stdout, "Pending job %s deleted successfully.\n", jobID)
}

func deletePendingJobsByTag(host, tag string) {
	u, err := url.Parse(fmt.Sprintf("%s/jobs/pending", host))
	if err != nil {
		fmt.Fprintf(stdout, "Failed to parse URL: %v\n", err)
		exitFunc(1)
		return
	}

	q := u.Query()
	q.Set("tag", tag)
	u.RawQuery = q.Encode()

	req, err := http.NewRequest(http.MethodDelete, u.String(), nil)
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
		fmt.Fprintf(stdout, "Failed to delete pending jobs by tag: %s\n", strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	var result struct {
		Deleted int `json:"deleted"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		fmt.Fprintf(stdout, "Failed to decode response: %v\n", err)
		exitFunc(1)
		return
	}

	fmt.Fprintf(stdout, "Successfully deleted %d pending jobs by tag.\n", result.Deleted)
}

func deletePendingJobsByMatch(host, match string) {
	u, err := url.Parse(fmt.Sprintf("%s/jobs/pending", host))
	if err != nil {
		fmt.Fprintf(stdout, "Failed to parse URL: %v\n", err)
		exitFunc(1)
		return
	}

	q := u.Query()
	q.Set("match", match)
	u.RawQuery = q.Encode()

	req, err := http.NewRequest(http.MethodDelete, u.String(), nil)
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
		fmt.Fprintf(stdout, "Failed to delete pending jobs by match: %s\n", strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	var result struct {
		Deleted int `json:"deleted"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		fmt.Fprintf(stdout, "Failed to decode response: %v\n", err)
		exitFunc(1)
		return
	}

	fmt.Fprintf(stdout, "Successfully deleted %d pending jobs by match.\n", result.Deleted)
}

func deletePendingJobsByGroup(host, group string) {
	u, err := url.Parse(fmt.Sprintf("%s/jobs/pending", host))
	if err != nil {
		fmt.Fprintf(stdout, "Failed to parse URL: %v\n", err)
		exitFunc(1)
		return
	}

	q := u.Query()
	q.Set("group", group)
	u.RawQuery = q.Encode()

	req, err := http.NewRequest(http.MethodDelete, u.String(), nil)
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
		fmt.Fprintf(stdout, "Failed to delete pending jobs by concurrency group: %s\n", strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	var result struct {
		Deleted int `json:"deleted"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		fmt.Fprintf(stdout, "Failed to decode response: %v\n", err)
		exitFunc(1)
		return
	}

	fmt.Fprintf(stdout, "Successfully deleted %d pending jobs by concurrency group.\n", result.Deleted)
}

func deletePendingJobsOlderThan(host, olderThan string) {
	u, err := url.Parse(fmt.Sprintf("%s/jobs/pending", host))
	if err != nil {
		fmt.Fprintf(stdout, "Failed to parse URL: %v\n", err)
		exitFunc(1)
		return
	}

	q := u.Query()
	q.Set("older_than", olderThan)
	u.RawQuery = q.Encode()

	req, err := http.NewRequest(http.MethodDelete, u.String(), nil)
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
		fmt.Fprintf(stdout, "Failed to delete pending jobs older than %s: %s\n", olderThan, strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	var result struct {
		Deleted int `json:"deleted"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		fmt.Fprintf(stdout, "Failed to decode response: %v\n", err)
		exitFunc(1)
		return
	}

	fmt.Fprintf(stdout, "Successfully deleted %d pending jobs older than %s.\n", result.Deleted, olderThan)
}
