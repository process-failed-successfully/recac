package main

import (
	"context"
	"encoding/json"
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
	Short: "Estimate a TODO item using AI",
	Long:  `Reads a specific TODO item from TODO.md, identifies the file and context (if available), and uses the AI agent to provide an estimation.`,
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

	// 2. Parse [File:Line] context if available
	var fileContext string
	re := regexp.MustCompile(`\[([^]]+):(\d+)\]`)
	matches := re.FindStringSubmatch(taskLine)

	if len(matches) >= 3 {
		filePath := matches[1]
		lineNum, _ := strconv.Atoi(matches[2])
		fmt.Fprintf(cmd.OutOrStdout(), "Found context in %s at line %d\n", filePath, lineNum)

		contentBytes, err := os.ReadFile(filePath)
		if err == nil {
			content := string(contentBytes)
			fileContext = fmt.Sprintf("File: %s\nLine: %d\nContent:\n'''\n%s\n'''", filePath, lineNum, content)
		} else {
			fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to read context file %s: %v\n", filePath, err)
		}
	} else {
		fmt.Fprintln(cmd.OutOrStdout(), "No file context found in task. Estimating based on description only.")
	}

	// 3. Construct Prompt
	ctx := context.Background()
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get cwd: %w", err)
	}

	provider := viper.GetString("provider")
	model := viper.GetString("model")
	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-todo-estimate")
	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}

	prompt := fmt.Sprintf(`You are a pragmatic Senior Software Engineer.
Your goal is to ESTIMATE the effort required for the following task found in TODO.md.

Task: "%s"

%s

Provide a realistic estimation. Be conservative. Consider testing, documentation, and potential side effects.

Return the result as a raw JSON object with the following structure:
{
  "summary": "Brief summary of the approach",
  "complexity": "Low|Medium|High",
  "story_points": <integer_fibonacci_sequence>,
  "estimated_hours": "range (e.g. 4-6h)",
  "risks": ["risk 1", "risk 2"],
  "implementation_steps": ["step 1", "step 2"]
}

Do not wrap the JSON in markdown code blocks. Just return the raw JSON string.`,
		taskLine,
		func() string {
			if fileContext != "" {
				return "Context Codebase:\n" + fileContext
			}
			return "No specific code context provided. Base estimate on general best practices."
		}())

	fmt.Fprintln(cmd.OutOrStdout(), "🤖 Crunching numbers (this may take a moment)...")

	// 4. Call Agent
	resp, err := ag.Send(ctx, prompt)
	if err != nil {
		return fmt.Errorf("agent failed: %w", err)
	}

	// 5. Parse Response
	jsonStr := utils.CleanJSONBlock(resp)
	var result EstimateResult
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		// Fallback
		fmt.Fprintf(cmd.ErrOrStderr(), "Warning: Failed to parse JSON response: %v\n", err)
		fmt.Fprintln(cmd.OutOrStdout(), "\nRaw Response:")
		fmt.Fprintln(cmd.OutOrStdout(), resp)
		return nil
	}

	// 6. Output
	PrintEstimateReport(cmd, result)

	return nil
}
