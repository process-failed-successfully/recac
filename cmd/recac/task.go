package main

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"recac/internal/orchestrator"

	"github.com/spf13/cobra"
)

var (
	workFile string
)

var taskCmd = &cobra.Command{
	Use:   "task",
	Short: "Manage local task file for orchestrator",
	Long:  `Manage the JSON file used by the orchestrator in 'file' polling mode. Allows adding and listing tasks.`,
}

var taskAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a new task",
	RunE:  runTaskAdd,
}

var taskListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all tasks",
	RunE:  runTaskList,
}

func init() {
	rootCmd.AddCommand(taskCmd)
	taskCmd.AddCommand(taskAddCmd)
	taskCmd.AddCommand(taskListCmd)

	taskCmd.PersistentFlags().StringVar(&workFile, "file", "work_items.json", "Path to the work items JSON file")

	taskAddCmd.Flags().String("id", "", "Task ID (auto-generated if empty)")
	taskAddCmd.Flags().String("summary", "", "Task summary")
	taskAddCmd.Flags().String("description", "", "Task description")
	taskAddCmd.Flags().String("repo-url", "", "Repository URL")
	taskAddCmd.MarkFlagRequired("summary")
}

func runTaskAdd(cmd *cobra.Command, args []string) error {
	id, _ := cmd.Flags().GetString("id")
	summary, _ := cmd.Flags().GetString("summary")
	description, _ := cmd.Flags().GetString("description")
	repoURL, _ := cmd.Flags().GetString("repo-url")

	if id == "" {
		id = fmt.Sprintf("TASK-%d", time.Now().Unix())
	}

	newItem := orchestrator.WorkItem{
		ID:          id,
		Summary:     summary,
		Description: description,
		RepoURL:     repoURL,
		EnvVars:     make(map[string]string),
	}

	items, err := readWorkItems(workFile)
	if err != nil {
		return err
	}

	items = append(items, newItem)

	if err := writeWorkItems(workFile, items); err != nil {
		return err
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Added task: %s\n", id)
	return nil
}

func runTaskList(cmd *cobra.Command, args []string) error {
	items, err := readWorkItems(workFile)
	if err != nil {
		return err
	}

	if len(items) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No tasks found.")
		return nil
	}

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "ID\tSUMMARY\tREPO URL")
	for _, item := range items {
		fmt.Fprintf(w, "%s\t%s\t%s\n", item.ID, item.Summary, item.RepoURL)
	}
	w.Flush()
	return nil
}

func readWorkItems(path string) ([]orchestrator.WorkItem, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return []orchestrator.WorkItem{}, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	var items []orchestrator.WorkItem
	if len(data) == 0 {
		return []orchestrator.WorkItem{}, nil
	}

	if err := json.Unmarshal(data, &items); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}
	return items, nil
}

func writeWorkItems(path string, items []orchestrator.WorkItem) error {
	data, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal items: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}
