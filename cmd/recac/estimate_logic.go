package main

import (
	"context"
	"encoding/json"
	"fmt"
	"recac/internal/agent"
	"recac/internal/utils"
)

// EstimateResult represents the structured output from the AI
type EstimateResult struct {
	Summary             string   `json:"summary"`
	Complexity          string   `json:"complexity"`      // Low, Medium, High
	StoryPoints         int      `json:"story_points"`    // Fibonacci: 1, 2, 3, 5, 8, 13, 21
	EstimatedHours      string   `json:"estimated_hours"` // e.g. "2-4h"
	Risks               []string `json:"risks"`
	ImplementationSteps []string `json:"implementation_steps"`
}

// EstimateTaskWithAgent uses the provided agent to estimate the complexity of a task.
// Returns raw response, parsed result, and error.
func EstimateTaskWithAgent(ctx context.Context, agentClient agent.Agent, taskDescription string, codebaseContext string) (string, *EstimateResult, error) {
	prompt := fmt.Sprintf(`You are a pragmatic Senior Software Engineer.
Your goal is to ESTIMATE the effort required for the following task.

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
		taskDescription,
		func() string {
			if codebaseContext != "" {
				return "Context Codebase:\n" + codebaseContext
			}
			return "No specific code context provided. Base estimate on general best practices."
		}())

	resp, err := agentClient.Send(ctx, prompt)
	if err != nil {
		return "", nil, fmt.Errorf("agent failed to generate estimate: %w", err)
	}

	jsonStr := utils.CleanJSONBlock(resp)
	var result EstimateResult
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return resp, nil, fmt.Errorf("failed to parse JSON response: %w", err)
	}

	return resp, &result, nil
}
