package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"
	"time"

	"recac/internal/orchestrator"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var taskCmd = &cobra.Command{
	Use:   "task",
	Short: "Manage orchestrator work items",
	Long:  `Manage work items for the orchestrator, specifically for file and file-dir pollers.`,
}

var taskAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a new work item",
	RunE:  runTaskAdd,
}

var taskListCmd = &cobra.Command{
	Use:   "list",
	Short: "List work items",
	RunE:  runTaskList,
}

func init() {
	rootCmd.AddCommand(taskCmd)
	taskCmd.AddCommand(taskAddCmd)
	taskCmd.AddCommand(taskListCmd)

	// Add flags
	taskAddCmd.Flags().String("summary", "", "Summary of the task (required)")
	taskAddCmd.MarkFlagRequired("summary")
	taskAddCmd.Flags().String("description", "", "Detailed description of the task")
	taskAddCmd.Flags().String("repo", "", "Repository URL to clone")
	taskAddCmd.Flags().StringToString("env", nil, "Environment variables (key=value)")
	taskAddCmd.Flags().String("poller", "file", "Poller type (file, file-dir)")
	taskAddCmd.Flags().String("file", "work_items.json", "Work items file (for 'file' poller)")
	taskAddCmd.Flags().String("dir", ".", "Directory for work item files (for 'file-dir' poller)")

	taskListCmd.Flags().String("poller", "file", "Poller type (file, file-dir)")
	taskListCmd.Flags().String("file", "work_items.json", "Work items file (for 'file' poller)")
	taskListCmd.Flags().String("dir", ".", "Directory for work item files (for 'file-dir' poller)")
}

func runTaskAdd(cmd *cobra.Command, args []string) error {
	summary, _ := cmd.Flags().GetString("summary")
	description, _ := cmd.Flags().GetString("description")
	repo, _ := cmd.Flags().GetString("repo")
	env, _ := cmd.Flags().GetStringToString("env")
	poller, _ := cmd.Flags().GetString("poller")
	file, _ := cmd.Flags().GetString("file")
	dir, _ := cmd.Flags().GetString("dir")

	// Determine poller from config if not set (though default is "file")
	if !cmd.Flags().Changed("poller") && viper.IsSet("orchestrator.poller") {
		poller = viper.GetString("orchestrator.poller")
	}

	item := orchestrator.WorkItem{
		ID:          uuid.New().String(),
		Summary:     summary,
		Description: description,
		RepoURL:     repo,
		EnvVars:     env,
	}

	switch poller {
	case "file":
		// Load existing
		var items []orchestrator.WorkItem
		if _, err := os.Stat(file); err == nil {
			data, err := os.ReadFile(file)
			if err != nil {
				return fmt.Errorf("failed to read work file: %w", err)
			}
			if len(data) > 0 {
				if err := json.Unmarshal(data, &items); err != nil {
					return fmt.Errorf("failed to unmarshal work file: %w", err)
				}
			}
		}

		items = append(items, item)

		data, err := json.MarshalIndent(items, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal items: %w", err)
		}

		if err := os.WriteFile(file, data, 0644); err != nil {
			return fmt.Errorf("failed to write work file: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Added task %s to %s\n", item.ID, file)

	case "file-dir":
		filename := fmt.Sprintf("task-%d-%s.json", time.Now().Unix(), item.ID[:8])
		path := filepath.Join(dir, filename)

		data, err := json.MarshalIndent(item, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal item: %w", err)
		}

		if err := os.WriteFile(path, data, 0644); err != nil {
			return fmt.Errorf("failed to write task file: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Added task %s to %s\n", item.ID, path)

	default:
		return fmt.Errorf("poller type '%s' not supported for 'add' command yet", poller)
	}

	return nil
}

func runTaskList(cmd *cobra.Command, args []string) error {
	poller, _ := cmd.Flags().GetString("poller")
	file, _ := cmd.Flags().GetString("file")
	dir, _ := cmd.Flags().GetString("dir")

	if !cmd.Flags().Changed("poller") && viper.IsSet("orchestrator.poller") {
		poller = viper.GetString("orchestrator.poller")
	}

	var items []orchestrator.WorkItem

	switch poller {
	case "file":
		if _, err := os.Stat(file); os.IsNotExist(err) {
			fmt.Fprintf(cmd.OutOrStdout(), "No work file found at %s\n", file)
			return nil
		}

		data, err := os.ReadFile(file)
		if err != nil {
			return fmt.Errorf("failed to read work file: %w", err)
		}

		if err := json.Unmarshal(data, &items); err != nil {
			return fmt.Errorf("failed to unmarshal work file: %w", err)
		}

	case "file-dir":
		entries, err := os.ReadDir(dir)
		if err != nil {
			return fmt.Errorf("failed to read directory: %w", err)
		}

		for _, entry := range entries {
			if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
				continue
			}

			// Very simplistic check: assume any .json file in this dir is a task
			path := filepath.Join(dir, entry.Name())
			data, err := os.ReadFile(path)
			if err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to read %s: %v\n", path, err)
				continue
			}

			var item orchestrator.WorkItem
			if err := json.Unmarshal(data, &item); err != nil {
				// Might not be a task file, skip silently or warn
				continue
			}
			items = append(items, item)
		}

	default:
		return fmt.Errorf("poller type '%s' not supported for 'list' command yet", poller)
	}

	if len(items) == 0 {
		fmt.Println("No tasks found.")
		return nil
	}

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "ID\tSUMMARY\tREPO")
	for _, item := range items {
		// Truncate summary if too long
		summary := item.Summary
		if len(summary) > 50 {
			summary = summary[:47] + "..."
		}
		repo := item.RepoURL
		if repo == "" {
			repo = "-"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\n", item.ID[:8], summary, repo)
	}
	w.Flush()

	return nil
}
