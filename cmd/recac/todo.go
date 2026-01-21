package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

const todoFile = "TODO.md"

var todoCmd = &cobra.Command{
	Use:   "todo",
	Short: "Manage a simple local TODO list in TODO.md",
	Long:  `A lightweight task manager that stores tasks in a Markdown file (TODO.md) in the current directory.`,
}

var todoAddCmd = &cobra.Command{
	Use:   "add [task]",
	Short: "Add a new task",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		task := strings.Join(args, " ")
		return appendTask(task)
	},
}

var todoListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all tasks",
	RunE: func(cmd *cobra.Command, args []string) error {
		return listTasks(cmd)
	},
}

var todoDoneCmd = &cobra.Command{
	Use:   "done [index]",
	Short: "Mark a task as done",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		index, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("invalid index: %s", args[0])
		}
		return toggleTaskStatus(index, true)
	},
}

var todoUndoneCmd = &cobra.Command{
	Use:   "undone [index]",
	Short: "Mark a task as not done",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		index, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("invalid index: %s", args[0])
		}
		return toggleTaskStatus(index, false)
	},
}

var todoRmCmd = &cobra.Command{
	Use:   "rm [index]",
	Short: "Remove a task",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		index, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("invalid index: %s", args[0])
		}
		return removeTask(index)
	},
}

func init() {
	rootCmd.AddCommand(todoCmd)
	todoCmd.AddCommand(todoAddCmd)
	todoCmd.AddCommand(todoListCmd)
	todoCmd.AddCommand(todoDoneCmd)
	todoCmd.AddCommand(todoUndoneCmd)
	todoCmd.AddCommand(todoRmCmd)
}

func ensureTodoFile() error {
	if _, err := os.Stat(todoFile); os.IsNotExist(err) {
		return os.WriteFile(todoFile, []byte("# TODO\n\n"), 0644)
	}
	return nil
}

func appendTask(task string) error {
	if err := ensureTodoFile(); err != nil {
		return err
	}

	f, err := os.OpenFile(todoFile, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err := f.WriteString(fmt.Sprintf("- [ ] %s\n", task)); err != nil {
		return err
	}
	fmt.Printf("Added task: %s\n", task)
	return nil
}

func listTasks(cmd *cobra.Command) error {
	if err := ensureTodoFile(); err != nil {
		return err
	}

	lines, err := readLines(todoFile)
	if err != nil {
		return err
	}

	index := 1
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "- [ ]") {
			fmt.Fprintf(cmd.OutOrStdout(), "%d. [ ] %s\n", index, strings.TrimPrefix(trimmed, "- [ ] "))
			index++
		} else if strings.HasPrefix(trimmed, "- [x]") {
			fmt.Fprintf(cmd.OutOrStdout(), "%d. [x] %s\n", index, strings.TrimPrefix(trimmed, "- [x] "))
			index++
		}
	}
	if index == 1 {
		fmt.Fprintln(cmd.OutOrStdout(), "No tasks found.")
	}
	return nil
}

func toggleTaskStatus(targetIndex int, done bool) error {
	if err := ensureTodoFile(); err != nil {
		return err
	}

	lines, err := readLines(todoFile)
	if err != nil {
		return err
	}

	newLines := make([]string, 0, len(lines))
	currentIndex := 1
	found := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "- [ ]") || strings.HasPrefix(trimmed, "- [x]") {
			if currentIndex == targetIndex {
				content := ""
				if strings.HasPrefix(trimmed, "- [ ]") {
					content = strings.TrimPrefix(trimmed, "- [ ] ")
				} else {
					content = strings.TrimPrefix(trimmed, "- [x] ")
				}

				prefix := "- [ ]"
				if done {
					prefix = "- [x]"
				}
				newLines = append(newLines, fmt.Sprintf("%s %s", prefix, content))
				found = true
			} else {
				newLines = append(newLines, line)
			}
			currentIndex++
		} else {
			newLines = append(newLines, line)
		}
	}

	if !found {
		return fmt.Errorf("task index %d not found", targetIndex)
	}

	return writeLines(todoFile, newLines)
}

func removeTask(targetIndex int) error {
	if err := ensureTodoFile(); err != nil {
		return err
	}

	lines, err := readLines(todoFile)
	if err != nil {
		return err
	}

	newLines := make([]string, 0, len(lines))
	currentIndex := 1
	found := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "- [ ]") || strings.HasPrefix(trimmed, "- [x]") {
			if currentIndex == targetIndex {
				found = true
			} else {
				newLines = append(newLines, line)
			}
			currentIndex++
		} else {
			newLines = append(newLines, line)
		}
	}

	if !found {
		return fmt.Errorf("task index %d not found", targetIndex)
	}

	fmt.Printf("Removed task %d\n", targetIndex)
	return writeLines(todoFile, newLines)
}

