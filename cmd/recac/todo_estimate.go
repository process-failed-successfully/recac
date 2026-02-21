package main

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"recac/internal/utils"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var todoEstimateAll bool
var todoEstimateForce bool

var todoEstimateCmd = &cobra.Command{
	Use:   "estimate [index]",
	Short: "Estimate complexity for a task in TODO.md",
	Long: `Estimates the complexity of a task in TODO.md using AI.
If the task contains a file reference (e.g. [path/to/file:123]), that file is used as context.
The estimate is appended to the task in TODO.md.

Examples:
  recac todo estimate 1
  recac todo estimate --all`,
	RunE: runTodoEstimate,
}

func init() {
	todoCmd.AddCommand(todoEstimateCmd)
	todoEstimateCmd.Flags().BoolVar(&todoEstimateAll, "all", false, "Estimate all unestimated tasks")
	todoEstimateCmd.Flags().BoolVar(&todoEstimateForce, "force", false, "Re-estimate tasks that already have an estimate")
}

func runTodoEstimate(cmd *cobra.Command, args []string) error {
	if err := ensureTodoFile(); err != nil {
		return err
	}

	lines, err := utils.ReadLines(todoFile)
	if err != nil {
		return err
	}

	var targetIndices []int

	if todoEstimateAll {
		// Find all tasks
		idx := 1
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "- [ ]") || strings.HasPrefix(trimmed, "- [x]") {
				// Check if already estimated
				if todoEstimateForce || !strings.Contains(trimmed, "(Est:") {
					targetIndices = append(targetIndices, idx)
				}
				idx++
			}
		}
		if len(targetIndices) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No tasks to estimate.")
			return nil
		}
	} else {
		if len(args) != 1 {
			return fmt.Errorf("accepts 1 arg(s), received %d. Use --all to estimate all tasks", len(args))
		}
		index, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("invalid index: %s", args[0])
		}
		targetIndices = append(targetIndices, index)
	}

	// Initialize Agent
	ctx := cmd.Context()
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get cwd: %w", err)
	}
	provider := viper.GetString("provider")
	model := viper.GetString("model")
	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-todo-estimate")
	if err != nil {
		return fmt.Errorf("failed to initialize agent: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Estimating %d task(s)...\n", len(targetIndices))

	// Regex to extract file path: [path/to/file:123]
	reFile := regexp.MustCompile(`\[([^]]+):(\d+)\]`)

	for _, index := range targetIndices {
		// Re-read file to get fresh content (in case of interleaved writes, though we are sequential here)
		// Actually, we can just read the specific line from our 'lines' cache but for modification we need to be careful.
		// Since we are modifying the file, let's use modifyTask logic but we need to read the task content first.

		// Find the task line
		var taskLine string
		var found bool

		// We reload lines every time to ensure we have the correct line content if indices shift (though they shouldn't with modifyTask)
		// But modifyTask relies on index which depends on the number of tasks.
		// If we modify a line, the number of tasks stays the same, so indices are stable.

		currentLines, err := utils.ReadLines(todoFile)
		if err != nil {
			return err
		}

		currIdx := 1
		for _, line := range currentLines {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "- [ ]") || strings.HasPrefix(trimmed, "- [x]") {
				if currIdx == index {
					taskLine = trimmed
					found = true
					break
				}
				currIdx++
			}
		}

		if !found {
			fmt.Fprintf(cmd.ErrOrStderr(), "Task %d not found, skipping.\n", index)
			continue
		}

		// Check if already estimated (double check for race or if we are iterating)
		if !todoEstimateForce && strings.Contains(taskLine, "(Est:") {
			fmt.Fprintf(cmd.OutOrStdout(), "Task %d already estimated, skipping.\n", index)
			continue
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Analyzing Task %d: %s...\n", index, shortenTask(taskLine))

		// Extract context file
		var contextCode string
		matches := reFile.FindStringSubmatch(taskLine)
		if len(matches) >= 2 {
			filePath := matches[1]
			content, err := os.ReadFile(filePath)
			if err == nil {
				contextCode = fmt.Sprintf("File: %s\n```\n%s\n```", filePath, string(content))
			} else {
				fmt.Fprintf(cmd.ErrOrStderr(), "  Warning: could not read context file %s: %v\n", filePath, err)
			}
		}

		// Estimate
		// Clean task description: remove [- [ ] ] and [file:line]
		cleanDesc := cleanTaskDescription(taskLine)

		_, res, err := EstimateTaskWithAgent(ctx, ag, cleanDesc, contextCode)
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "  Failed to estimate task %d: %v\n", index, err)
			continue
		}

		// Update TODO.md
		estimateStr := fmt.Sprintf(" **(Est: %dpts, %s)**", res.StoryPoints, res.Complexity)

		err = modifyTask(index, func(line string) (string, bool) {
			// Remove existing estimate if force
			if todoEstimateForce {
				line = removeExistingEstimate(line)
			}
			return line + estimateStr, true
		})

		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "  Failed to update TODO.md for task %d: %v\n", index, err)
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "  ✅ Estimated: %d pts (%s)\n", res.StoryPoints, res.Complexity)
		}
	}

	return nil
}

func shortenTask(task string) string {
	if len(task) > 50 {
		return task[:47] + "..."
	}
	return task
}

func cleanTaskDescription(line string) string {
	line = strings.TrimSpace(line)
	// Remove "- [ ]" or "- [x]" (allow flexible spacing)
	if strings.HasPrefix(line, "- [ ]") {
		line = strings.TrimPrefix(line, "- [ ]")
	} else if strings.HasPrefix(line, "- [x]") {
		line = strings.TrimPrefix(line, "- [x]")
	}

	line = strings.TrimSpace(line)

	// Remove [file:line] prefix if present
	// We want to keep the "Keyword: Content" part
	// Regex: ^\[[^]]+\]\s*
	re := regexp.MustCompile(`^\[[^]]+\]\s*`)
	line = re.ReplaceAllString(line, "")

	return line
}

func removeExistingEstimate(line string) string {
	// Remove **(Est: ...)**
	re := regexp.MustCompile(` \*\*\(Est:.*?\)\*\*`)
	return re.ReplaceAllString(line, "")
}
