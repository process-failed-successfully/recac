package main

import (
	"recac/internal/utils"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

// todoFilename is a variable to allow overriding in tests
var todoFilename = "TODO.md"

// TodoTask represents a task in the todo list
type TodoTask struct {
	Description string
	Done        bool
}

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
	if _, err := os.Stat(todoFilename); os.IsNotExist(err) {
		return os.WriteFile(todoFilename, []byte("# TODO\n\n"), 0644)
	}
	return nil
}

// loadTasks reads the TODO file and returns a list of tasks
func loadTasks() ([]TodoTask, error) {
	if err := ensureTodoFile(); err != nil {
		return nil, err
	}

	lines, err := utils.ReadLines(todoFilename)
	if err != nil {
		return nil, err
	}

	var tasks []TodoTask
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "- [ ]") {
			tasks = append(tasks, TodoTask{
				Description: strings.TrimPrefix(trimmed, "- [ ] "),
				Done:        false,
			})
		} else if strings.HasPrefix(trimmed, "- [x]") {
			tasks = append(tasks, TodoTask{
				Description: strings.TrimPrefix(trimmed, "- [x] "),
				Done:        true,
			})
		}
	}
	return tasks, nil
}

// saveTasks writes the list of tasks to the TODO file, preserving non-task lines
func saveTasks(tasks []TodoTask) error {
	if err := ensureTodoFile(); err != nil {
		return err
	}

	lines, err := utils.ReadLines(todoFilename)
	if err != nil {
		return err
	}

	var newLines []string
	taskIndex := 0

	// First pass: replace existing tasks or append non-task lines
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "- [ ]") || strings.HasPrefix(trimmed, "- [x]") {
			if taskIndex < len(tasks) {
				task := tasks[taskIndex]
				prefix := "- [ ]"
				if task.Done {
					prefix = "- [x]"
				}
				newLines = append(newLines, fmt.Sprintf("%s %s", prefix, task.Description))
				taskIndex++
			}
			// If we run out of tasks, we skip the remaining lines (deletion)
		} else {
			newLines = append(newLines, line)
		}
	}

	// Append any remaining new tasks (additions)
	for i := taskIndex; i < len(tasks); i++ {
		task := tasks[i]
		prefix := "- [ ]"
		if task.Done {
			prefix = "- [x]"
		}
		newLines = append(newLines, fmt.Sprintf("%s %s", prefix, task.Description))
	}

	return utils.WriteLines(todoFilename, newLines)
}

func appendTask(taskDesc string) error {
	tasks, err := loadTasks()
	if err != nil {
		return err
	}
	tasks = append(tasks, TodoTask{Description: taskDesc, Done: false})
	if err := saveTasks(tasks); err != nil {
		return err
	}
	fmt.Printf("Added task: %s\n", taskDesc)
	return nil
}

func listTasks(cmd *cobra.Command) error {
	tasks, err := loadTasks()
	if err != nil {
		return err
	}

	if len(tasks) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No tasks found.")
		return nil
	}

	for i, task := range tasks {
		status := "[ ]"
		if task.Done {
			status = "[x]"
		}
		fmt.Fprintf(cmd.OutOrStdout(), "%d. %s %s\n", i+1, status, task.Description)
	}
	return nil
}

func toggleTaskStatus(targetIndex int, done bool) error {
	tasks, err := loadTasks()
	if err != nil {
		return err
	}

	if targetIndex < 1 || targetIndex > len(tasks) {
		return fmt.Errorf("invalid index: %d", targetIndex)
	}

	tasks[targetIndex-1].Done = done
	return saveTasks(tasks)
}

func removeTask(targetIndex int) error {
	tasks, err := loadTasks()
	if err != nil {
		return err
	}

	if targetIndex < 1 || targetIndex > len(tasks) {
		return fmt.Errorf("invalid index: %d", targetIndex)
	}

	// Remove element at index-1
	tasks = append(tasks[:targetIndex-1], tasks[targetIndex:]...)
	if err := saveTasks(tasks); err != nil {
		return err
	}
	fmt.Printf("Removed task %d\n", targetIndex)
	return nil
}
