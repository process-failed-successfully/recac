package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"recac/internal/agent"
	"recac/internal/agent/prompts"
	"recac/internal/db"
	"recac/internal/utils"
	"os"
	"path/filepath"
)

// RunPlanOnly executes the planning phase and exits.
func (s *Session) RunPlanOnly(ctx context.Context) error {
	s.Logger.Info("Running in Plan-Only mode")

	// 1. Read Spec
	spec, err := s.ReadSpec()
	if err != nil {
		return fmt.Errorf("failed to read spec: %w", err)
	}

	// 2. Load Prompt
	prompt, err := prompts.GetPrompt(prompts.PlanOnly, map[string]string{
		"spec": spec,
	})
	if err != nil {
		return fmt.Errorf("failed to load plan_only prompt: %w", err)
	}

	// 3. Send to Agent
	s.Logger.Info("Generating plan...")
	response, err := s.Agent.Send(ctx, prompt)
	if err != nil {
		return fmt.Errorf("agent failed to generate plan: %w", err)
	}

	// 4. Save Plan
	planPath := filepath.Join(s.Workspace, "PLAN.md")
	if err := os.WriteFile(planPath, []byte(response), 0644); err != nil {
		return fmt.Errorf("failed to save PLAN.md: %w", err)
	}

	// 5. Output to logs
	fmt.Println("--- PLAN START ---")
	fmt.Println(response)
	fmt.Println("--- PLAN END ---")
	s.Logger.Info("Plan generated successfully", "path", planPath)

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
