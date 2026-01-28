package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

var todoSortCmd = &cobra.Command{
	Use:   "sort",
	Short: "Sort tasks in TODO.md by priority and status",
	Long: `Sorts the TODO.md file.
Priorities:
1. FIXME, BUG
2. TODO
3. HACK, NOTE
4. Others

Completed tasks ([x]) are moved to the bottom.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runTodoSort(cmd)
	},
}

func init() {
	todoCmd.AddCommand(todoSortCmd)
}

type ParsedTask struct {
	Original  string
	CleanText string // Text without "- [ ] "
	IsDone    bool
	Priority  int
}

func runTodoSort(cmd *cobra.Command) error {
	if err := ensureTodoFile(); err != nil {
		return err
	}

	lines, err := readLines(todoFile)
	if err != nil {
		return err
	}

	var header []string
	var tasks []*ParsedTask

	seenFirstTask := false
	var currentTask *ParsedTask

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		isTask := strings.HasPrefix(trimmed, "- [ ]") || strings.HasPrefix(trimmed, "- [x]")

		if !seenFirstTask {
			if isTask {
				seenFirstTask = true
				currentTask = parseTask(line)
				tasks = append(tasks, currentTask)
			} else {
				header = append(header, line)
			}
		} else {
			if isTask {
				currentTask = parseTask(line)
				tasks = append(tasks, currentTask)
			} else {
				// Attach non-task lines to the previous task (e.g. notes, spacing)
				if currentTask != nil {
					currentTask.Original += "\n" + line
				}
			}
		}
	}

	sort.SliceStable(tasks, func(i, j int) bool {
		t1 := tasks[i]
		t2 := tasks[j]

		// 1. Completion status (Not done comes first)
		if t1.IsDone != t2.IsDone {
			return !t1.IsDone
		}

		// 2. Priority (Lower is better)
		if t1.Priority != t2.Priority {
			return t1.Priority < t2.Priority
		}

		// 3. Alphabetical (for stability/predictability)
		return t1.CleanText < t2.CleanText
	})

	// Reconstruct file
	var newLines []string
	newLines = append(newLines, header...)
	for _, t := range tasks {
		newLines = append(newLines, t.Original)
	}

	if err := writeLines(todoFile, newLines); err != nil {
		return err
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Sorted %d tasks in %s\n", len(tasks), todoFile)
	return nil
}

func parseTask(line string) *ParsedTask {
	trimmed := strings.TrimSpace(line)
	isDone := strings.HasPrefix(trimmed, "- [x]")

	// Remove prefix
	clean := ""
	if isDone {
		clean = strings.TrimPrefix(trimmed, "- [x]")
	} else {
		clean = strings.TrimPrefix(trimmed, "- [ ]")
	}
	clean = strings.TrimSpace(clean)

	// Determine priority based on keywords in the line
	// We look for [FIXME], [BUG], FIXME:, BUG: or just the words
	priority := 4 // Default

	upper := strings.ToUpper(clean)
	if strings.Contains(upper, "FIXME") || strings.Contains(upper, "BUG") {
		priority = 1
	} else if strings.Contains(upper, "TODO") {
		priority = 2
	} else if strings.Contains(upper, "HACK") || strings.Contains(upper, "NOTE") {
		priority = 3
	}

	return &ParsedTask{
		Original:  line, // Keep original indentation if any? No, todo scan writes strict lines.
		CleanText: clean,
		IsDone:    isDone,
		Priority:  priority,
	}
}
