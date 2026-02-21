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

// GeneratePlanOnly generates an architectural plan based on the spec.
func GeneratePlanOnly(ctx context.Context, a agent.Agent, spec string) (string, error) {
	prompt, err := prompts.GetPrompt(prompts.PlanOnly, map[string]string{
		"spec": spec,
	})
	if err != nil {
		return "", fmt.Errorf("failed to load plan_only prompt: %w", err)
	}

	response, err := a.Send(ctx, prompt)
	if err != nil {
		return "", fmt.Errorf("agent failed to generate plan: %w", err)
	}

	// Clean response (remove markdown code blocks if present, though we asked for markdown)
	// Actually, if the agent wraps it in ```markdown ... ```, we might want to extract it.
	// But usually CleanCodeBlock handles that. However, the prompt asks for "Markdown document".
	// Sometimes agents wrap it in ```markdown`.
	// Let's use CleanCodeBlock just in case, but only if it detects a block.
	// utils.CleanCodeBlock returns the content inside the block.

	cleaned := utils.CleanCodeBlock(response)
	return cleaned, nil
}
