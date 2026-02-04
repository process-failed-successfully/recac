package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"recac/internal/orchestrator"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var submitCmd = &cobra.Command{
	Use:   "submit",
	Short: "Submit a new work item",
	Long:  `Submit a new work item to the orchestrator via the configured poller (currently supports 'file' and 'file-dir').`,
	RunE: func(cmd *cobra.Command, args []string) error {
		pollerType := viper.GetString("orchestrator.poller")
		summary, _ := cmd.Flags().GetString("summary")
		description, _ := cmd.Flags().GetString("description")
		repoURL, _ := cmd.Flags().GetString("repo-url")
		id, _ := cmd.Flags().GetString("id")

		if summary == "" {
			return fmt.Errorf("summary is required")
		}

		if id == "" {
			id = fmt.Sprintf("TASK-%s", uuid.New().String()[:8])
		}

		item := orchestrator.WorkItem{
			ID:          id,
			Summary:     summary,
			Description: description,
			RepoURL:     repoURL,
			EnvVars:     make(map[string]string),
		}

		switch pollerType {
		case "file-dir":
			watchDir := viper.GetString("orchestrator.watch_dir")
			if watchDir == "" {
				return fmt.Errorf("watch-dir is not configured")
			}
			return submitToFileDir(watchDir, item)
		case "file", "filesystem":
			workFile := viper.GetString("orchestrator.work_file")
			if workFile == "" {
				return fmt.Errorf("work-file is not configured")
			}
			return submitToFile(workFile, item)
		default:
			return fmt.Errorf("submit is not supported for poller type: %s", pollerType)
		}
	},
}

func init() {
	rootCmd.AddCommand(submitCmd)

	submitCmd.Flags().String("summary", "", "Task summary")
	submitCmd.Flags().String("description", "", "Task description")
	submitCmd.Flags().String("repo-url", "", "Repository URL")
	submitCmd.Flags().String("id", "", "Custom Task ID (optional)")
}

func submitToFileDir(dir string, item orchestrator.WorkItem) error {
	// Ensure directory exists
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create watch directory: %w", err)
	}

	fileName := fmt.Sprintf("%s.json", item.ID)
	path := filepath.Join(dir, fileName)

	data, err := json.MarshalIndent(item, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal item: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	fmt.Printf("Submitted task %s to %s\n", item.ID, path)
	return nil
}

func submitToFile(path string, item orchestrator.WorkItem) error {
	var items []orchestrator.WorkItem

	// Read existing
	if _, err := os.Stat(path); err == nil {
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read existing work file: %w", err)
		}
		if len(data) > 0 {
			if err := json.Unmarshal(data, &items); err != nil {
				return fmt.Errorf("failed to parse existing work file: %w", err)
			}
		}
	}

	// Append
	items = append(items, item)

	// Write back
	data, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal items: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write work file: %w", err)
	}

	fmt.Printf("Submitted task %s to %s\n", item.ID, path)
	return nil
}
