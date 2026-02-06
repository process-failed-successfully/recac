package main

import (
	"encoding/json"
	"fmt"
	"os"
	"recac/internal/orchestrator"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var taskCmd = &cobra.Command{
	Use:   "task",
	Short: "Manage local work items",
	Long:  `Add, list, and clear work items for the local file poller.`,
}

var taskAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a new task",
	RunE:  runTaskAdd,
}

var taskListCmd = &cobra.Command{
	Use:   "list",
	Short: "List tasks",
	RunE:  runTaskList,
}

var taskClearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Clear all tasks",
	RunE:  runTaskClear,
}

func init() {
	rootCmd.AddCommand(taskCmd)
	taskCmd.AddCommand(taskAddCmd)
	taskCmd.AddCommand(taskListCmd)
	taskCmd.AddCommand(taskClearCmd)

	taskAddCmd.Flags().String("summary", "", "Task summary")
	taskAddCmd.Flags().String("desc", "", "Task description")
	taskAddCmd.Flags().String("repo", "", "Repository URL")
	_ = taskAddCmd.MarkFlagRequired("summary")
}

func getWorkFile() string {
	return viper.GetString("orchestrator.work_file")
}

func loadWorkItems() ([]orchestrator.WorkItem, error) {
	path := getWorkFile()
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return []orchestrator.WorkItem{}, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read work file: %w", err)
	}

	var items []orchestrator.WorkItem
	if len(data) == 0 {
		return []orchestrator.WorkItem{}, nil
	}

	if err := json.Unmarshal(data, &items); err != nil {
		return nil, fmt.Errorf("failed to parse work file: %w", err)
	}
	return items, nil
}

func saveWorkItems(items []orchestrator.WorkItem) error {
	path := getWorkFile()
	data, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal items: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}

func runTaskAdd(cmd *cobra.Command, args []string) error {
	summary, _ := cmd.Flags().GetString("summary")
	desc, _ := cmd.Flags().GetString("desc")
	repo, _ := cmd.Flags().GetString("repo")

	items, err := loadWorkItems()
	if err != nil {
		return err
	}

	// Simple ID generation
	id := fmt.Sprintf("TASK-%d", time.Now().Unix())

	newItem := orchestrator.WorkItem{
		ID:          id,
		Summary:     summary,
		Description: desc,
		RepoURL:     repo,
		EnvVars:     make(map[string]string),
	}

	items = append(items, newItem)

	if err := saveWorkItems(items); err != nil {
		return err
	}

	fmt.Printf("Added task %s: %s\n", id, summary)
	return nil
}

func runTaskList(cmd *cobra.Command, args []string) error {
	items, err := loadWorkItems()
	if err != nil {
		return err
	}

	if len(items) == 0 {
		fmt.Println("No tasks found.")
		return nil
	}

	fmt.Printf("%-20s %-50s %s\n", "ID", "SUMMARY", "REPO")
	fmt.Println("--------------------------------------------------------------------------------")
	for _, item := range items {
		repo := item.RepoURL
		if repo == "" {
			repo = "(none)"
		}
		summary := item.Summary
		if len(summary) > 47 {
			summary = summary[:47] + "..."
		}
		fmt.Printf("%-20s %-50s %s\n", item.ID, summary, repo)
	}
	return nil
}

func runTaskClear(cmd *cobra.Command, args []string) error {
	if err := saveWorkItems([]orchestrator.WorkItem{}); err != nil {
		return err
	}
	fmt.Println("Cleared all tasks.")
	return nil
}
