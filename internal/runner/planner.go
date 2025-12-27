package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"recac/internal/agent"
	"recac/internal/agent/prompts"
	"regexp"
	"strings"
)

type Feature struct {
	Category    string   `json:"category"`
	Description string   `json:"description"`
	Steps       []string `json:"steps"`
	Passes      bool     `json:"passes"`
}

// GenerateFeatureList asks the agent to decompose the spec into features.
func GenerateFeatureList(ctx context.Context, a agent.Agent, spec string) ([]Feature, error) {
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

	var features []Feature
	if err := json.Unmarshal([]byte(cleanedResponse), &features); err != nil {
		return nil, fmt.Errorf("failed to parse agent response: %w\nResponse: %s", err, response)
	}

	return features, nil
}

func cleanJSON(input string) string {
	// Remove ```json and ``` lines
	re := regexp.MustCompile("(?s)```json(.*?)```")
	match := re.FindStringSubmatch(input)
	if len(match) > 1 {
		return strings.TrimSpace(match[1])
	}
	// Try without json tag
	re2 := regexp.MustCompile("(?s)```(.*?)```")
	match2 := re2.FindStringSubmatch(input)
	if len(match2) > 1 {
		return strings.TrimSpace(match2[1])
	}
	return strings.TrimSpace(input)
}
