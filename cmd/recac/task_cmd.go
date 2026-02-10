package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"recac/internal/cmdutils"
	"recac/internal/orchestrator"
	"recac/internal/telemetry"
	"text/tabwriter"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func init() {
	rootCmd.AddCommand(taskCmd)

	// Task List
	taskCmd.AddCommand(taskListCmd)

	// Task Add
	taskCmd.AddCommand(taskAddCmd)
	taskAddCmd.Flags().String("summary", "", "Summary of the task (required)")
	taskAddCmd.Flags().String("description", "", "Description of the task")
	taskAddCmd.Flags().String("repo-url", "", "Repository URL for the task")
	taskAddCmd.Flags().String("priority", "Medium", "Priority of the task (High, Medium, Low)")
	taskAddCmd.Flags().String("project", "", "Jira Project Key (e.g. PROJ)")
	taskAddCmd.MarkFlagRequired("summary")
}

var taskCmd = &cobra.Command{
	Use:   "task",
	Short: "Manage orchestrator tasks",
	Long:  `List and add tasks to the orchestrator queue (File, FileDir, or Jira).`,
}

var taskListCmd = &cobra.Command{
	Use:   "list",
	Short: "List pending tasks",
	Long:  `Lists tasks that the orchestrator would currently pick up from the configured source.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		logger := telemetry.NewLogger(viper.GetBool("verbose"), "recac-cli", false)

		pollerType := viper.GetString("orchestrator.poller")
		if pollerType == "" {
			pollerType = "jira" // Default
		}

		var poller orchestrator.Poller
		var err error

		switch pollerType {
		case "file-dir":
			watchDir := viper.GetString("orchestrator.watch_dir")
			if watchDir == "" {
				return fmt.Errorf("orchestrator.watch_dir is required for file-dir poller")
			}
			poller, err = orchestrator.NewFileDirPoller(watchDir)
			if err != nil {
				return fmt.Errorf("failed to create file-dir poller: %w", err)
			}
			fmt.Printf("Source: FileDir (%s)\n", watchDir)

		case "file", "filesystem":
			workFile := viper.GetString("orchestrator.work_file")
			if workFile == "" {
				workFile = "work_items.json"
			}
			poller = orchestrator.NewFilePoller(workFile)
			fmt.Printf("Source: File (%s)\n", workFile)

		case "jira":
			jClient, err := cmdutils.GetJiraClient(ctx)
			if err != nil {
				return fmt.Errorf("failed to create Jira client: %w", err)
			}
			jql := viper.GetString("orchestrator.jira_query")
			label := viper.GetString("orchestrator.jira_label")
			if jql == "" && label != "" {
				jql = fmt.Sprintf("labels = \"%s\" AND statusCategory != Done ORDER BY created ASC", label)
			}
			poller = orchestrator.NewJiraPoller(jClient, jql)
			fmt.Printf("Source: Jira (JQL: %s)\n", jql)

		default:
			return fmt.Errorf("unknown poller type: %s", pollerType)
		}

		// Poll tasks
		items, err := poller.Poll(ctx, logger)
		if err != nil {
			return fmt.Errorf("failed to poll tasks: %w", err)
		}

		if len(items) == 0 {
			fmt.Println("No pending tasks found.")
			return nil
		}

		// Print items
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "ID\tSUMMARY\tREPO")
		for _, item := range items {
			repo := item.RepoURL
			if repo == "" {
				repo = "-"
			}
			fmt.Fprintf(w, "%s\t%s\t%s\n", item.ID, item.Summary, repo)
		}
		w.Flush()

		return nil
	},
}

var taskAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a new task",
	Long:  `Adds a new task to the configured source (File, FileDir, or Jira).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		summary, _ := cmd.Flags().GetString("summary")
		description, _ := cmd.Flags().GetString("description")
		repoURL, _ := cmd.Flags().GetString("repo-url")
		priority, _ := cmd.Flags().GetString("priority")

		pollerType := viper.GetString("orchestrator.poller")
		if pollerType == "" {
			pollerType = "jira"
		}

		switch pollerType {
		case "file":
			workFile := viper.GetString("orchestrator.work_file")
			if workFile == "" {
				workFile = "work_items.json"
			}
			return addToFile(workFile, summary, description, repoURL, priority)

		case "file-dir":
			watchDir := viper.GetString("orchestrator.watch_dir")
			if watchDir == "" {
				return fmt.Errorf("orchestrator.watch_dir is required for file-dir poller")
			}
			return addToFileDir(watchDir, summary, description, repoURL, priority)

		case "jira":
			project, _ := cmd.Flags().GetString("project")
			if project == "" {
				project = os.Getenv("JIRA_PROJECT_KEY")
			}
			if project == "" {
				// Try to get first project key as fallback? No, explicit is better for writes.
				return fmt.Errorf("--project flag or JIRA_PROJECT_KEY env var is required for Jira")
			}

			jClient, err := cmdutils.GetJiraClient(ctx)
			if err != nil {
				return fmt.Errorf("failed to create Jira client: %w", err)
			}

			labels := viper.GetStringSlice("orchestrator.jira_label") // Add poller label if any
			if len(labels) == 0 {
				l := viper.GetString("orchestrator.jira_label")
				if l != "" {
					labels = []string{l}
				}
			}

			// Note: Priority is not yet supported for Jira creation via this command.
			key, err := jClient.CreateTicket(ctx, project, summary, description, "Task", labels)
			if err != nil {
				return fmt.Errorf("failed to create Jira ticket: %w", err)
			}
			fmt.Printf("Created Jira ticket: %s\n", key)
			return nil

		default:
			return fmt.Errorf("unknown poller type: %s", pollerType)
		}
	},
}

func addToFile(path, summary, description, repoURL, priority string) error {
	// Read existing items
	var items []orchestrator.WorkItem
	if _, err := os.Stat(path); err == nil {
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read work file: %w", err)
		}
		if len(data) > 0 {
			if err := json.Unmarshal(data, &items); err != nil {
				return fmt.Errorf("failed to parse work file: %w", err)
			}
		}
	}

	envVars := map[string]string{
		"CREATED_AT": time.Now().Format(time.RFC3339),
	}
	if priority != "" {
		envVars["PRIORITY"] = priority
	}

	newItem := orchestrator.WorkItem{
		ID:          uuid.New().String(),
		Summary:     summary,
		Description: description,
		RepoURL:     repoURL,
		EnvVars:     envVars,
	}

	items = append(items, newItem)

	data, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal items: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write work file: %w", err)
	}

	fmt.Printf("Added task to %s (ID: %s)\n", path, newItem.ID)
	return nil
}

func addToFileDir(dir, summary, description, repoURL, priority string) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create watch directory: %w", err)
	}

	envVars := map[string]string{
		"CREATED_AT": time.Now().Format(time.RFC3339),
	}
	if priority != "" {
		envVars["PRIORITY"] = priority
	}

	id := uuid.New().String()
	newItem := orchestrator.WorkItem{
		ID:          id,
		Summary:     summary,
		Description: description,
		RepoURL:     repoURL,
		EnvVars:     envVars,
	}

	data, err := json.MarshalIndent(newItem, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal item: %w", err)
	}

	filename := filepath.Join(dir, fmt.Sprintf("%s.json", id))
	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write task file: %w", err)
	}

	fmt.Printf("Added task to %s\n", filename)
	return nil
}
