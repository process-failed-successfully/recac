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
	Short: "Estimate complexity for a TODO item using AI",
	Long:  `Reads a specific TODO item from TODO.md, identifies the file context, and uses the AI agent to estimate effort, complexity, and risks.`,
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
	// Expected format: "- [ ] [path/to/file:123] Keyword: Content"
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
	ctx := context.Background() // Or cmd.Context()
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

Context File: %s
Line: %d
Content:
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

	fmt.Fprintln(cmd.OutOrStdout(), "🤖 Crunching numbers (this may take a moment)...")

	// 5. Call Agent
	resp, err := ag.Send(ctx, prompt)
	if err != nil {
		return fmt.Errorf("agent failed: %w", err)
	}

	// 6. Parse Response
	jsonStr := utils.CleanJSONBlock(resp)
	var result EstimateResult
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		// Fallback: Just print raw output if parsing fails
		fmt.Fprintf(cmd.ErrOrStderr(), "Warning: Failed to parse JSON response: %v\n", err)
		fmt.Fprintln(cmd.OutOrStdout(), "\nRaw Response:")
		fmt.Fprintln(cmd.OutOrStdout(), resp)
		return nil
	}

	// 7. Print Report
	PrintEstimateReport(cmd, result)

	return nil
}
