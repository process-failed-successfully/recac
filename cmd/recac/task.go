package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"recac/internal/cmdutils"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// dependency injection for testing
var getJiraClientFunc = cmdutils.GetJiraClient

var (
	taskDescription string
	taskPriority    string
	taskType        string
	taskPoller      string
	taskLabels      []string
)

var taskCmd = &cobra.Command{
	Use:   "task",
	Short: "Manage tasks (submit, list)",
	Long:  `Manage tasks for the autonomous agent. Tasks can be submitted to Jira or a local file queue depending on configuration.`,
}

var taskSubmitCmd = &cobra.Command{
	Use:   "submit [title]",
	Short: "Submit a new task",
	Args:  cobra.ExactArgs(1),
	RunE:  runTaskSubmit,
}

var taskListCmd = &cobra.Command{
	Use:   "list",
	Short: "List pending tasks",
	RunE:  runTaskList,
}

func init() {
	rootCmd.AddCommand(taskCmd)
	taskCmd.AddCommand(taskSubmitCmd)
	taskCmd.AddCommand(taskListCmd)

	// Submit flags
	taskSubmitCmd.Flags().StringVarP(&taskDescription, "description", "d", "", "Task description")
	taskSubmitCmd.Flags().StringVarP(&taskPriority, "priority", "p", "", "Task priority")
	taskSubmitCmd.Flags().StringVarP(&taskType, "type", "t", "Task", "Jira Issue Type (e.g. Task, Bug)")
	taskSubmitCmd.Flags().StringVar(&taskPoller, "poller", "", "Override poller type (jira, file, file-dir)")
	taskSubmitCmd.Flags().StringSliceVarP(&taskLabels, "label", "l", nil, "Additional labels")
}

type LocalTask struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Status      string    `json:"status"`
	Priority    string    `json:"priority,omitempty"`
	Labels      []string  `json:"labels,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

func runTaskSubmit(cmd *cobra.Command, args []string) error {
	title := args[0]
	poller := taskPoller
	if poller == "" {
		poller = viper.GetString("orchestrator.poller")
	}
	if poller == "" {
		poller = "jira" // Default
	}

	switch poller {
	case "jira":
		return submitToJira(cmd, title)
	case "file":
		return submitToFile(cmd, title)
	case "file-dir":
		return submitToFileDir(cmd, title)
	default:
		return fmt.Errorf("unknown poller type: %s", poller)
	}
}

func submitToJira(cmd *cobra.Command, title string) error {
	ctx := context.Background()
	client, err := getJiraClientFunc(ctx)
	if err != nil {
		return fmt.Errorf("failed to initialize Jira client: %w", err)
	}

	projectKey := viper.GetString("jira.project_key")
	if projectKey == "" {
		projectKey = os.Getenv("JIRA_PROJECT_KEY")
	}
	if projectKey == "" {
		// Try to fetch first project
		pk, err := client.GetFirstProjectKey(ctx)
		if err != nil {
			return fmt.Errorf("JIRA_PROJECT_KEY not set and failed to fetch projects: %w", err)
		}
		projectKey = pk
	}

	baseLabel := viper.GetString("orchestrator.jira_label")
	if baseLabel == "" {
		baseLabel = "recac-agent"
	}
	labels := append([]string{baseLabel}, taskLabels...)

	// Append priority to description since we don't handle priority field explicitly yet
	desc := taskDescription
	if taskPriority != "" {
		desc += fmt.Sprintf("\n\nPriority: %s", taskPriority)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Creating Jira ticket in project %s...\n", projectKey)
	key, err := client.CreateTicket(ctx, projectKey, title, desc, taskType, labels)
	if err != nil {
		return fmt.Errorf("failed to create ticket: %w", err)
	}

	baseURL := viper.GetString("jira.url")
	if baseURL == "" {
		baseURL = os.Getenv("JIRA_URL")
	}
	fmt.Fprintf(cmd.OutOrStdout(), "✅ Task submitted successfully: %s\nLink: %s/browse/%s\n", key, baseURL, key)
	return nil
}

func submitToFile(cmd *cobra.Command, title string) error {
	file := viper.GetString("orchestrator.work_file")
	if file == "" {
		file = "work_items.json"
	}

	var tasks []LocalTask
	if _, err := os.Stat(file); err == nil {
		content, err := os.ReadFile(file)
		if err != nil {
			return fmt.Errorf("failed to read work file: %w", err)
		}
		if len(content) > 0 {
			if err := json.Unmarshal(content, &tasks); err != nil {
				return fmt.Errorf("failed to parse work file: %w", err)
			}
		}
	}

	newTask := LocalTask{
		ID:          fmt.Sprintf("TASK-%d", time.Now().Unix()),
		Title:       title,
		Description: taskDescription,
		Status:      "pending",
		Priority:    taskPriority,
		Labels:      taskLabels,
		CreatedAt:   time.Now(),
	}

	tasks = append(tasks, newTask)

	data, err := json.MarshalIndent(tasks, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal tasks: %w", err)
	}

	if err := os.WriteFile(file, data, 0644); err != nil {
		return fmt.Errorf("failed to write work file: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "✅ Task submitted to %s: %s\n", file, newTask.ID)
	return nil
}

func submitToFileDir(cmd *cobra.Command, title string) error {
	dir := viper.GetString("orchestrator.watch_dir")
	if dir == "" {
		return fmt.Errorf("orchestrator.watch_dir must be configured for file-dir poller")
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to ensure watch directory: %w", err)
	}

	newTask := LocalTask{
		ID:          fmt.Sprintf("TASK-%d", time.Now().UnixNano()),
		Title:       title,
		Description: taskDescription,
		Status:      "pending",
		Priority:    taskPriority,
		Labels:      taskLabels,
		CreatedAt:   time.Now(),
	}

	filename := fmt.Sprintf("task-%d.json", time.Now().UnixNano())
	path := filepath.Join(dir, filename)

	data, err := json.MarshalIndent(newTask, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal task: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write task file: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "✅ Task file created: %s\n", path)
	return nil
}

func runTaskList(cmd *cobra.Command, args []string) error {
	poller := taskPoller
	if poller == "" {
		poller = viper.GetString("orchestrator.poller")
	}
	if poller == "" {
		poller = "jira"
	}

	switch poller {
	case "jira":
		return listJiraTasks(cmd)
	case "file":
		return listFileTasks(cmd)
	case "file-dir":
		return listFileDirTasks(cmd)
	default:
		return fmt.Errorf("unknown poller type: %s", poller)
	}
}

func listJiraTasks(cmd *cobra.Command) error {
	ctx := context.Background()
	client, err := getJiraClientFunc(ctx)
	if err != nil {
		return fmt.Errorf("failed to initialize Jira client: %w", err)
	}

	label := viper.GetString("orchestrator.jira_label")
	if label == "" {
		label = "recac-agent"
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Searching Jira tickets with label '%s'...\n", label)
	issues, err := client.LoadLabelIssues(ctx, label)
	if err != nil {
		return fmt.Errorf("failed to load issues: %w", err)
	}

	if len(issues) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No tasks found.")
		return nil
	}

	for _, issue := range issues {
		key := issue["key"].(string)
		fields := issue["fields"].(map[string]interface{})
		summary := fields["summary"].(string)
		statusMap := fields["status"].(map[string]interface{})
		status := statusMap["name"].(string)
		fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\n", key, status, summary)
	}
	return nil
}

func listFileTasks(cmd *cobra.Command) error {
	file := viper.GetString("orchestrator.work_file")
	if file == "" {
		file = "work_items.json"
	}

	if _, err := os.Stat(file); os.IsNotExist(err) {
		fmt.Fprintln(cmd.OutOrStdout(), "No work file found.")
		return nil
	}

	content, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("failed to read work file: %w", err)
	}

	var tasks []LocalTask
	if err := json.Unmarshal(content, &tasks); err != nil {
		return fmt.Errorf("failed to parse work file: %w", err)
	}

	for _, t := range tasks {
		fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\n", t.ID, t.Status, t.Title)
	}
	return nil
}

func listFileDirTasks(cmd *cobra.Command) error {
	dir := viper.GetString("orchestrator.watch_dir")
	if dir == "" {
		return fmt.Errorf("orchestrator.watch_dir not configured")
	}

	files, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("failed to read dir: %w", err)
	}

	for _, f := range files {
		if strings.HasSuffix(f.Name(), ".json") {
			content, err := os.ReadFile(filepath.Join(dir, f.Name()))
			if err != nil {
				continue
			}
			var t LocalTask
			if err := json.Unmarshal(content, &t); err == nil {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\n", t.ID, t.Status, t.Title)
			}
		}
	}
	return nil
}
