package main

import (
	"recac/internal/utils"
	"fmt"
	"os"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
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

var todoUiCmd = &cobra.Command{
	Use:   "ui",
	Short: "Interactive TODO list manager",
	RunE: func(cmd *cobra.Command, args []string) error {
		model, err := NewTodoModel()
		if err != nil {
			return err
		}
		p := tea.NewProgram(model, tea.WithAltScreen())
		if _, err := p.Run(); err != nil {
			return err
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(todoCmd)
	todoCmd.AddCommand(todoAddCmd)
	todoCmd.AddCommand(todoListCmd)
	todoCmd.AddCommand(todoDoneCmd)
	todoCmd.AddCommand(todoUndoneCmd)
	todoCmd.AddCommand(todoRmCmd)
	todoCmd.AddCommand(todoUiCmd)
}

func ensureTodoFile() error {
	if _, err := os.Stat(todoFile); os.IsNotExist(err) {
		return os.WriteFile(todoFile, []byte("# TODO\n\n"), 0644)
	}
	return nil
}

func appendTask(taskDesc string) error {
	tasks, err := loadTasks()
	if err != nil {
		return err
	}

	tasks = append(tasks, Task{
		Desc: taskDesc,
		Done: false,
	})

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

	for i, t := range tasks {
		prefix := "[ ]"
		if t.Done {
			prefix = "[x]"
		}
		fmt.Fprintf(cmd.OutOrStdout(), "%d. %s %s\n", i+1, prefix, t.Desc)
	}
	return nil
}

func toggleTaskStatus(targetIndex int, done bool) error {
	tasks, err := loadTasks()
	if err != nil {
		return err
	}

	if targetIndex < 1 || targetIndex > len(tasks) {
		return fmt.Errorf("task index %d not found", targetIndex)
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
		return fmt.Errorf("task index %d not found", targetIndex)
	}

	tasks = append(tasks[:targetIndex-1], tasks[targetIndex:]...)

	if err := saveTasks(tasks); err != nil {
		return err
	}
	fmt.Printf("Removed task %d\n", targetIndex)
	return nil
}

type Task struct {
	Desc string
	Done bool
}

func (t Task) FilterValue() string {
	return t.Desc
}

func (t Task) Title() string {
	if t.Done {
		return "[x] " + t.Desc
	}
	return "[ ] " + t.Desc
}

func (t Task) Description() string {
	return ""
}

func loadTasks() ([]Task, error) {
	if err := ensureTodoFile(); err != nil {
		return nil, err
	}
	lines, err := utils.ReadLines(todoFile)
	if err != nil {
		return nil, err
	}

	var tasks []Task
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "- [ ]") {
			tasks = append(tasks, Task{
				Desc: strings.TrimPrefix(trimmed, "- [ ] "),
				Done: false,
			})
		} else if strings.HasPrefix(trimmed, "- [x]") {
			tasks = append(tasks, Task{
				Desc: strings.TrimPrefix(trimmed, "- [x] "),
				Done: true,
			})
		}
	}
	return tasks, nil
}

func saveTasks(tasks []Task) error {
	lines := []string{"# TODO", ""}
	for _, t := range tasks {
		prefix := "- [ ]"
		if t.Done {
			prefix = "- [x]"
		}
		lines = append(lines, fmt.Sprintf("%s %s", prefix, t.Desc))
	}
	return utils.WriteLines(todoFile, lines)
}
