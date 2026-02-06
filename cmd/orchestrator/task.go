package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"recac/internal/orchestrator"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func init() {
	rootCmd.AddCommand(taskCmd)
	taskCmd.AddCommand(taskAddCmd)
	taskCmd.AddCommand(taskListCmd)
	taskCmd.AddCommand(taskClearCmd)

	taskAddCmd.Flags().String("summary", "", "Task summary")
	taskAddCmd.Flags().String("description", "", "Task description")
	taskAddCmd.Flags().String("repo-url", "", "Target repository URL")
	taskAddCmd.Flags().StringSlice("env", []string{}, "Environment variables (KEY=VALUE)")
}

var taskCmd = &cobra.Command{
	Use:   "task",
	Short: "Manage local work items",
	Long:  `Manage the local work items file used by the 'file' poller.`,
}

var taskAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a new task",
	RunE: func(cmd *cobra.Command, args []string) error {
		file := viper.GetString("orchestrator.work_file")
		summary, _ := cmd.Flags().GetString("summary")
		desc, _ := cmd.Flags().GetString("description")
		repo, _ := cmd.Flags().GetString("repo-url")
		envSlice, _ := cmd.Flags().GetStringSlice("env")

		if summary == "" {
			return fmt.Errorf("Error: --summary is required")
		}
		if repo == "" {
			return fmt.Errorf("Error: --repo-url is required")
		}

		envVars := make(map[string]string)
		for _, e := range envSlice {
			parts := strings.SplitN(e, "=", 2)
			if len(parts) == 2 {
				envVars[parts[0]] = parts[1]
			}
		}

		// Generate ID
		id := fmt.Sprintf("TASK-%d", time.Now().UnixNano())

		item := orchestrator.WorkItem{
			ID:          id,
			Summary:     summary,
			Description: desc,
			RepoURL:     repo,
			EnvVars:     envVars,
		}

		items, err := loadWorkItems(file)
		if err != nil {
			return fmt.Errorf("Error loading items: %v", err)
		}

		items = append(items, item)

		if err := saveWorkItems(file, items); err != nil {
			return fmt.Errorf("Error saving items: %v", err)
		}

		fmt.Printf("Added task %s\n", id)
		return nil
	},
}

var taskListCmd = &cobra.Command{
	Use:   "list",
	Short: "List tasks",
	RunE: func(cmd *cobra.Command, args []string) error {
		file := viper.GetString("orchestrator.work_file")
		items, err := loadWorkItems(file)
		if err != nil {
			return fmt.Errorf("Error loading items: %v", err)
		}

		if len(items) == 0 {
			fmt.Println("No tasks found.")
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "ID\tSUMMARY\tREPO")
		for _, item := range items {
			fmt.Fprintf(w, "%s\t%s\t%s\n", item.ID, item.Summary, item.RepoURL)
		}
		w.Flush()
		return nil
	},
}

var taskClearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Clear all tasks",
	RunE: func(cmd *cobra.Command, args []string) error {
		file := viper.GetString("orchestrator.work_file")
		if err := saveWorkItems(file, []orchestrator.WorkItem{}); err != nil {
			return fmt.Errorf("Error clearing items: %v", err)
		}
		fmt.Println("Cleared all tasks.")
		return nil
	},
}

func loadWorkItems(path string) ([]orchestrator.WorkItem, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return []orchestrator.WorkItem{}, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var items []orchestrator.WorkItem
	if err := json.Unmarshal(data, &items); err != nil {
		return nil, err
	}
	return items, nil
}

func saveWorkItems(path string, items []orchestrator.WorkItem) error {
	data, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
