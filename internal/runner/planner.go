package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"recac/internal/agent"
	"recac/internal/agent/prompts"
	"recac/internal/db"
	"recac/internal/utils"
)

// GeneratePlanOnly generates a plan without executing it.
func GeneratePlanOnly(ctx context.Context, a agent.Agent, spec string, workspace string) error {
	prompt, err := prompts.GetPrompt(prompts.PlanOnly, map[string]string{
		"spec": spec,
	})
	if err != nil {
		return fmt.Errorf("failed to load plan_only prompt: %w", err)
	}

	fmt.Println("Generating implementation plan...")
	response, err := a.Send(ctx, prompt)
	if err != nil {
		return fmt.Errorf("agent failed to generate plan: %w", err)
	}

	// Remove markdown code blocks if the agent wrapped the whole response in one
	// But usually for markdown response we keep it as markdown.
	// Just ensure it's not wrapped in JSON or something weird.
	// The prompt asks for Markdown.

	planPath := filepath.Join(workspace, "PLAN.md")
	if err := os.WriteFile(planPath, []byte(response), 0644); err != nil {
		return fmt.Errorf("failed to write PLAN.md: %w", err)
	}

	fmt.Printf("Plan generated successfully: %s\n", planPath)
	return nil
}

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
