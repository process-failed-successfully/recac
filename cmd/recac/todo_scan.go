package main

import (
	"fmt"
	"os"
	"recac/internal/analysis/todo"
	"recac/internal/utils"
	"strings"

	"github.com/spf13/cobra"
)

// TodoItem is an alias for the shared todo.Item type.
type TodoItem = todo.Item

var todoScanCmd = &cobra.Command{
	Use:   "scan [path]",
	Short: "Scan codebase for TODOs and add them to TODO.md",
	Long:  `Scans the specified path (defaults to current directory) for comments starting with TODO, FIXME, BUG, HACK, or NOTE and adds them to the TODO.md file.`,
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		root := "."
		if len(args) > 0 {
			root = args[0]
		}
		return scanAndAddTodos(cmd, root)
	},
}

func init() {
	// todoCmd is defined in todo.go
	todoCmd.AddCommand(todoScanCmd)
}

func scanAndAddTodos(cmd *cobra.Command, root string) error {
	tasks, err := ScanForTodos(root)
	if err != nil {
		return err
	}

	if len(tasks) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No new TODOs found.")
		return nil
	}

	count, err := addTasksToTodoFile(tasks)
	if err != nil {
		return err
	}

	if count > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "Added %d new tasks to TODO.md\n", count)
	} else {
		fmt.Fprintln(cmd.OutOrStdout(), "No new tasks added (all duplicates).")
	}

	return nil
}

// ScanForTodos scans the root directory for TODO items using the shared logic.
func ScanForTodos(root string) ([]TodoItem, error) {
	return todo.Scan(root, utils.DefaultIgnoreMap())
}

func addTasksToTodoFile(newTasks []TodoItem) (int, error) {
	if err := ensureTodoFile(); err != nil {
		return 0, err
	}

	lines, err := readLines(todoFile)
	if err != nil {
		return 0, err
	}

	existingTasks := make(map[string]bool)
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "- [ ] ") {
			existingTasks[strings.TrimPrefix(trimmed, "- [ ] ")] = true
		} else if strings.HasPrefix(trimmed, "- [x] ") {
			existingTasks[strings.TrimPrefix(trimmed, "- [x] ")] = true
		}
	}

	addedCount := 0
	f, err := os.OpenFile(todoFile, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	for _, item := range newTasks {
		if !existingTasks[item.Raw] {
			if _, err := f.WriteString(fmt.Sprintf("- [ ] %s\n", item.Raw)); err != nil {
				return addedCount, err
			}
			addedCount++
		}
	}

	return addedCount, nil
}
