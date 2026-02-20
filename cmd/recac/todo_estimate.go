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
	Short: "Estimate complexity for a specific TODO item",
	Long:  `Analyzes a specific TODO item from TODO.md using AI.
It reads the referenced file context and provides an estimation of complexity, time, and risks.`,
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
	// We could add flags here if needed, e.g. --json
	todoEstimateCmd.Flags().BoolVar(&estimateJson, "json", false, "Output results as JSON")
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
	var lineNum int
	var content string

	if len(matches) >= 3 {
		filePath = matches[1]
		lineNum, _ = strconv.Atoi(matches[2])

		// 3. Read target file
		contentBytes, err := os.ReadFile(filePath)
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to read referenced file %s: %v\n", filePath, err)
			content = "File could not be read."
		} else {
			content = string(contentBytes)
		}
	} else {
		// Fallback if no file context found in TODO
		fmt.Fprintf(cmd.ErrOrStderr(), "Warning: could not identify file and line in task: %s\n", taskLine)
		content = "No file context available."
		filePath = "Unknown"
	}

	fmt.Fprintf(cmd.OutOrStdout(), "🔍 Estimating TODO #%d: %s\n", index, taskLine)
	if filePath != "Unknown" {
		fmt.Fprintf(cmd.OutOrStdout(), "   Context: %s:%d\n", filePath, lineNum)
	}

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

	prompt := fmt.Sprintf(`You are a pragmatic Senior Software Engineer.
Your goal is to ESTIMATE the effort required for the following TODO task.

Task: "%s"
File: %s
Line: %d

File Content Context:
'''
%s
'''

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
		taskLine, filePath, lineNum, content)

	fmt.Fprintln(cmd.OutOrStdout(), "🤖 Crunching numbers...")

	// 5. Call Agent
	resp, err := ag.Send(ctx, prompt)
	if err != nil {
		return fmt.Errorf("agent failed: %w", err)
	}

	// 6. Parse and Print
	jsonStr := utils.CleanJSONBlock(resp)
	var result EstimateResult
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Warning: Failed to parse JSON response: %v\n", err)
		fmt.Fprintln(cmd.OutOrStdout(), "\nRaw Response:")
		fmt.Fprintln(cmd.OutOrStdout(), resp)
		return nil
	}

	// Reuse the output logic from estimate.go
	if estimateJson {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}

	PrintEstimateReport(cmd, result)

	return nil
}
