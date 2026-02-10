package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"recac/internal/orchestrator"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var taskCmd = &cobra.Command{
	Use:   "task",
	Short: "Manage Orchestrator work items",
	Long:  `Manage tasks for the Orchestrator (add, list) when using 'file' or 'file-dir' pollers.`,
}

var taskAddCmd = &cobra.Command{
	Use:   "add [summary]",
	Short: "Add a new task",
	Args:  cobra.ExactArgs(1),
	RunE:  runTaskAdd,
}

var taskListCmd = &cobra.Command{
	Use:   "list",
	Short: "List pending tasks",
	RunE:  runTaskList,
}

func init() {
	rootCmd.AddCommand(taskCmd)
	taskCmd.AddCommand(taskAddCmd)
	taskCmd.AddCommand(taskListCmd)

	// Flags for taskCmd (apply to subcommands)
	taskCmd.PersistentFlags().String("poller", "", "Poller type: 'file' or 'file-dir' (defaults to config)")
	taskCmd.PersistentFlags().String("work-file", "", "Work items file (for 'file' poller)")
	taskCmd.PersistentFlags().String("watch-dir", "", "Directory to watch (for 'file-dir' poller)")

	// Bind flags to viper, but we must use a unique key or share with orchestrator?
	// The Orchestrator uses "orchestrator.poller", etc.
	// We want to use the SAME config.
	// But binding persistent flags here might conflict if rootCmd also binds?
	// rootCmd does NOT bind these specific flags. `orchestrateCmd` does.
	// So we can bind them here manually or just look them up.
	// To support CLI override, we should read flag first, then viper.

	// Flags for add
	taskAddCmd.Flags().StringP("description", "d", "", "Task description")
	taskAddCmd.Flags().StringP("repo", "r", "", "Target repository URL")
}

func getTaskConfig(cmd *cobra.Command) (string, string, string) {
	// 1. Check Flag
	poller, _ := cmd.Flags().GetString("poller")
	if poller == "" {
		// 2. Check Viper (orchestrator config)
		poller = viper.GetString("orchestrator.poller")
	}

	workFile, _ := cmd.Flags().GetString("work-file")
	if workFile == "" {
		workFile = viper.GetString("orchestrator.work_file")
	}

	watchDir, _ := cmd.Flags().GetString("watch-dir")
	if watchDir == "" {
		watchDir = viper.GetString("orchestrator.watch_dir")
	}

	return poller, workFile, watchDir
}

func runTaskAdd(cmd *cobra.Command, args []string) error {
	summary := args[0]
	desc, _ := cmd.Flags().GetString("description")
	repo, _ := cmd.Flags().GetString("repo")

	poller, workFile, watchDir := getTaskConfig(cmd)

	if poller == "" {
		return fmt.Errorf("poller type not configured. Set 'orchestrator.poller' in config or use --poller")
	}

	// Create WorkItem
	item := orchestrator.WorkItem{
		ID:          uuid.NewString(),
		Summary:     summary,
		Description: desc,
		RepoURL:     repo,
		EnvVars:     make(map[string]string),
	}

	// Add Metadata
	item.EnvVars["CREATED_AT"] = time.Now().Format(time.RFC3339)
	item.EnvVars["SOURCE"] = "recac-cli"

	switch poller {
	case "file-dir":
		if watchDir == "" {
			return fmt.Errorf("watch-dir not configured")
		}
		// Ensure dir exists
		if err := os.MkdirAll(watchDir, 0755); err != nil {
			return fmt.Errorf("failed to create watch dir: %w", err)
		}
		filename := filepath.Join(watchDir, fmt.Sprintf("task-%s.json", item.ID))
		data, err := json.MarshalIndent(item, "", "  ")
		if err != nil {
			return err
		}
		if err := os.WriteFile(filename, data, 0644); err != nil {
			return fmt.Errorf("failed to write task file: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "✅ Task added: %s\n", filename)

	case "file":
		if workFile == "" {
			return fmt.Errorf("work-file not configured")
		}
		// Read existing
		var items []orchestrator.WorkItem
		if _, err := os.Stat(workFile); err == nil {
			data, err := os.ReadFile(workFile)
			if err != nil {
				return fmt.Errorf("failed to read work file: %w", err)
			}
			if len(data) > 0 {
				if err := json.Unmarshal(data, &items); err != nil {
					return fmt.Errorf("failed to parse work file: %w", err)
				}
			}
		} else if !os.IsNotExist(err) {
			return err
		}

		items = append(items, item)
		data, err := json.MarshalIndent(items, "", "  ")
		if err != nil {
			return err
		}
		if err := os.WriteFile(workFile, data, 0644); err != nil {
			return fmt.Errorf("failed to write work file: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "✅ Task added to %s\n", workFile)

	default:
		return fmt.Errorf("unsupported poller for manual task add: %s (only 'file' and 'file-dir' supported)", poller)
	}

	return nil
}

func runTaskList(cmd *cobra.Command, args []string) error {
	poller, workFile, watchDir := getTaskConfig(cmd)

	var items []orchestrator.WorkItem

	switch poller {
	case "file-dir":
		if watchDir == "" {
			return fmt.Errorf("watch-dir not configured")
		}
		entries, err := os.ReadDir(watchDir)
		if err != nil {
			if os.IsNotExist(err) {
				fmt.Println("No tasks found (directory does not exist).")
				return nil
			}
			return err
		}
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
				data, err := os.ReadFile(filepath.Join(watchDir, entry.Name()))
				if err == nil {
					var item orchestrator.WorkItem
					if json.Unmarshal(data, &item) == nil {
						items = append(items, item)
					}
				}
			}
		}

	case "file":
		if workFile == "" {
			return fmt.Errorf("work-file not configured")
		}
		if _, err := os.Stat(workFile); err == nil {
			data, err := os.ReadFile(workFile)
			if err == nil {
				json.Unmarshal(data, &items)
			}
		}

	default:
		return fmt.Errorf("unsupported poller: %s", poller)
	}

	if len(items) == 0 {
		fmt.Println("No pending tasks found.")
		return nil
	}

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "ID\tSUMMARY\tREPO")
	for _, item := range items {
		// Truncate ID for display
		shortID := item.ID
		if len(shortID) > 8 {
			shortID = shortID[:8]
		}
		fmt.Fprintf(w, "%s\t%s\t%s\n", shortID, item.Summary, item.RepoURL)
	}
	w.Flush()
	return nil
}
