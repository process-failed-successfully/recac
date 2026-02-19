package main

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"recac/internal/utils"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var todoSolveCmd = &cobra.Command{
	Use:   "solve [index]",
	Short: "Solve a TODO item using AI",
	Long:  `Reads a specific TODO item from TODO.md, identifies the file and context, and uses the AI agent to implement the solution.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		index, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("invalid index: %s", args[0])
		}
		return runTodoSolve(cmd, index)
	},
}

func init() {
	todoCmd.AddCommand(todoSolveCmd)
}

func runTodoSolve(cmd *cobra.Command, index int) error {
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
	// Expected format: "- [ ] [path/to/file:123] Keyword: Content"
	// Use [^]]+ to ensure we don't match across multiple brackets like [ ] [file:123]
	re := regexp.MustCompile(`\[([^]]+):(\d+)\]`)
	matches := re.FindStringSubmatch(taskLine)
	if len(matches) < 3 {
		return fmt.Errorf("could not identify file and line in task: %s\nMake sure the task was added via 'recac todo scan'", taskLine)
	}

	filePath := matches[1]
	lineNum, _ := strconv.Atoi(matches[2])

	fmt.Fprintf(cmd.OutOrStdout(), "Solving TODO in %s at line %d...\n", filePath, lineNum)

	// 3. Read target file
	contentBytes, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read target file %s: %w", filePath, err)
	}
	content := string(contentBytes)

	// 4. Construct Prompt
	ctx := context.Background()
	provider := viper.GetString("provider")
	model := viper.GetString("model")
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get cwd: %w", err)
	}

	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-todo-solve")
	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}

	prompt := fmt.Sprintf(`You are an expert software engineer.
I need you to resolve a TODO comment in the following file.

File: %s
Line: %d
Context from TODO task: "%s"

The file content is:
'''
%s
'''

INSTRUCTIONS:
1. Locate the TODO at the specified line.
2. Implement the missing logic or fix the issue described.
3. Remove the TODO comment.
4. Return the COMPLETE updated file content. Do not output diffs. Do not output markdown code fences (like '''go). Just the raw code.
`, filePath, lineNum, taskLine, content)

	fmt.Fprintln(cmd.OutOrStdout(), "Waiting for agent implementation...")

	// 5. Call Agent
	resp, err := ag.Send(ctx, prompt)
	if err != nil {
		return fmt.Errorf("agent failed: %w", err)
	}

	// 6. Clean and Write
	newContent := utils.CleanCodeBlock(resp)
	if newContent == "" {
		return fmt.Errorf("received empty response from agent")
	}

	if err := os.WriteFile(filePath, []byte(newContent), 0644); err != nil {
		return fmt.Errorf("failed to write updated file: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Updated %s\n", filePath)

	// 7. Mark as Done
	if err := toggleTaskStatus(index, true); err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to mark task as done: %v\n", err)
	} else {
		fmt.Fprintln(cmd.OutOrStdout(), "Task marked as done in TODO.md")
	}

	return nil
}
