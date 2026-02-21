package main

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"recac/internal/utils"

	"github.com/spf13/cobra"
)

var todoEstimateCmd = &cobra.Command{
	Use:   "estimate [index]",
	Short: "Estimate complexity for a TODO item",
	Long:  `Reads a specific TODO item from TODO.md, identifies the file and context, and uses the AI agent to estimate the effort.
The estimation (Story Points and Hours) will be appended to the task in TODO.md.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		index, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("invalid index: %s", args[0])
		}
		return runTodoEstimate(cmd, index)
	},
}

func init() {
	todoCmd.AddCommand(todoEstimateCmd)
}

func runTodoEstimate(cmd *cobra.Command, index int) error {
	// 1. Read TODO.md and get the task
	if err := ensureTodoFile(); err != nil {
		return err
	}
	lines, err := utils.ReadLines(todoFile)
	if err != nil {
		return err
	}

	taskLine := ""
	currentIndex := 1
	found := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "- [ ]") || strings.HasPrefix(trimmed, "- [x]") {
			if currentIndex == index {
				taskLine = trimmed
				found = true
				break
			}
			currentIndex++
		}
	}

	if !found {
		return fmt.Errorf("task index %d not found", index)
	}

	// 2. Parse [File:Line]
	re := regexp.MustCompile(`\[([^]]+):(\d+)\]`)
	matches := re.FindStringSubmatch(taskLine)

	var filePath string
	var contextMsg string

	if len(matches) >= 3 {
		filePath = matches[1]
		lineNum, _ := strconv.Atoi(matches[2])
		fmt.Fprintf(cmd.OutOrStdout(), "Analyzing TODO in %s at line %d...\n", filePath, lineNum)

		contentBytes, err := os.ReadFile(filePath)
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to read target file %s: %v\n", filePath, err)
			contextMsg = "File not found or unreadable."
		} else {
			fileContent := string(contentBytes)
			contextMsg = fmt.Sprintf("File: %s\nLine: %d\nContent:\n%s", filePath, lineNum, fileContent)
		}
	} else {
		fmt.Fprintln(cmd.OutOrStdout(), "No file context found in task. Estimating based on description only.")
		contextMsg = "No file context available."
	}

	// Extract description
	taskDescription := taskLine
	if idx := strings.Index(taskLine, "] "); idx != -1 {
		// Matches the first "] " which closes "- [ ] " or "- [x] "
		// But be careful if task content has "] "
		// Standard prefix is "- [ ] " (6 chars) or "- [x] " (6 chars)
		if len(taskLine) > 6 {
			taskDescription = taskLine[6:]
		}
	}

	fmt.Fprintln(cmd.OutOrStdout(), "🤖 Crunching numbers (this may take a moment)...")

	// 3. Call GetEstimation
	ctx := cmd.Context()
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	result, rawResp, err := GetEstimation(ctx, cwd, taskDescription, contextMsg)
	if err != nil {
		return err
	}

	if result == nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Warning: Failed to parse JSON response\n")
		fmt.Fprintln(cmd.OutOrStdout(), "\nRaw Response:")
		fmt.Fprintln(cmd.OutOrStdout(), rawResp)
		return nil
	}

	// 4. Output
	PrintEstimateReport(cmd, *result)

	// 5. Update TODO.md
	err = modifyTask(index, func(line string) (string, bool) {
		// Check if already estimated
		reEst := regexp.MustCompile(`\s*\(Est:.*\)$`)
		estStr := fmt.Sprintf(" (Est: %dpts, %s)", result.StoryPoints, result.EstimatedHours)

		if reEst.MatchString(line) {
			line = reEst.ReplaceAllString(line, estStr)
		} else {
			line = line + estStr
		}
		return line, true
	})

	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to update TODO.md: %v\n", err)
	} else {
		fmt.Fprintln(cmd.OutOrStdout(), "Updated task in TODO.md with estimate.")
	}

	return nil
}
