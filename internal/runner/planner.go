package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"recac/internal/agent"
	"recac/internal/agent/prompts"
	"recac/internal/db"
	"recac/internal/utils"
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
	cleanedResponse := utils.CleanJSONBlock(response)

	var featureList db.FeatureList
	if err := json.Unmarshal([]byte(cleanedResponse), &featureList); err != nil {
		return nil, fmt.Errorf("failed to parse agent response: %w\nResponse: %s", err, response)
	}

	return &featureList, nil
}
