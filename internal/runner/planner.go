package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"recac/internal/agent"
	"recac/internal/agent/prompts"
	"recac/internal/db"
	"regexp"
	"strings"
)

// GenerateFeatureList asks the agent to decompose the spec into features.
func GenerateFeatureList(ctx context.Context, a agent.Agent, spec string) (*db.FeatureList, error) {
	prompt, err := prompts.GetPrompt(prompts.Planner, map[string]string{
		"spec": spec,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to load planner prompt: %w", err)
	}

	response, err := a.Send(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("agent failed to generate plan: %w", err)
	}

	// Clean response (remove markdown code blocks if present)
	cleanedResponse := cleanJSON(response)

	var featureList db.FeatureList
	if err := json.Unmarshal([]byte(cleanedResponse), &featureList); err != nil {
		return nil, fmt.Errorf("failed to parse agent response: %w\nResponse: %s", err, response)
	}

	return &featureList, nil
}

var (
	reJSONBlock = regexp.MustCompile("(?s)```json(.*?)```")
	reBlock     = regexp.MustCompile("(?s)```(.*?)```")
)

func cleanJSON(input string) string {
	input = strings.TrimSpace(input)
	if input == "" {
		return ""
	}

	// Fast path: check for common markdown block wrappers without regex
	if strings.HasPrefix(input, "```json") && strings.HasSuffix(input, "```") {
		content := input[7 : len(input)-3]
		return strings.TrimSpace(content)
	}
	if strings.HasPrefix(input, "```") && strings.HasSuffix(input, "```") {
		// Might have a different language tag, find first newline
		content := input[3 : len(input)-3]
		if idx := strings.Index(content, "\n"); idx != -1 {
			// Check if first line doesn't contain space (likely a tag)
			tag := content[:idx]
			if !strings.Contains(tag, " ") {
				content = content[idx+1:]
			}
		}
		return strings.TrimSpace(content)
	}

	// Fallback to regex for more complex cases (like blocks hidden in text)
	match := reJSONBlock.FindStringSubmatch(input)
	if len(match) > 1 {
		return strings.TrimSpace(match[1])
	}
	// Try without json tag
	match2 := reBlock.FindStringSubmatch(input)
	if len(match2) > 1 {
		return strings.TrimSpace(match2[1])
	}
	return input
}
