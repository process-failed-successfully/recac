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

var todoEstimateCmd = &cobra.Command{
	Use:   "estimate [index]",
	Short: "Estimate complexity and plan for a TODO item",
	Long:  `Reads a specific TODO item from TODO.md, identifies the file and context, and uses the AI agent to estimate complexity, risks, and provide an implementation plan.`,
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
	if len(matches) < 3 {
		return fmt.Errorf("could not identify file and line in task: %s\nMake sure the task was added via 'recac todo scan'", taskLine)
	}

	filePath := matches[1]
	lineNum, _ := strconv.Atoi(matches[2])

	fmt.Fprintf(cmd.OutOrStdout(), "Estimating TODO in %s at line %d...\n", filePath, lineNum)

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

	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-todo-estimate")
	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}

	prompt := fmt.Sprintf(`You are an expert software engineer and project manager.
I need you to estimate the complexity and provide a plan to resolve a TODO comment in the following file.

File: %s
Line: %d
Context from TODO task: "%s"

The file content is:
'''
%s
'''

INSTRUCTIONS:
1. Analyze the TODO and the surrounding code.
2. Provide a Complexity Score (1-10) and Classification (Low/Medium/High).
3. Provide a step-by-step Implementation Plan.
4. Identify any Potential Risks (regressions, missing dependencies, ambiguity).
5. Format the output clearly with markdown.
`, filePath, lineNum, taskLine, content)

	fmt.Fprintln(cmd.OutOrStdout(), "Waiting for agent estimation...")

	// 5. Call Agent
	resp, err := ag.Send(ctx, prompt)
	if err != nil {
		return fmt.Errorf("agent failed: %w", err)
	}

	fmt.Fprintln(cmd.OutOrStdout(), "\n--- Estimation & Plan ---")
	fmt.Fprintln(cmd.OutOrStdout(), resp)
	fmt.Fprintln(cmd.OutOrStdout(), "-------------------------")

	return nil
}
