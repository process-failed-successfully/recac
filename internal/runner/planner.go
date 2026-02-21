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

// GeneratePlanOnly generates a detailed implementation plan without executing it.
func (s *Session) GeneratePlanOnly(ctx context.Context) error {
	s.Logger.Info("generating implementation plan (plan-only mode)")

	// 1. Get Spec
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
	fmt.Println("🤖 Analyzing specification and generating plan...")
	response, err := s.Agent.Send(ctx, prompt)
	if err != nil {
		return fmt.Errorf("agent failed to generate plan: %w", err)
	}

	// 4. Write to PLAN.md
	planPath := "PLAN.md" // In CWD (where user ran command)
	if s.Workspace != "" {
		planPath = filepath.Join(s.Workspace, "PLAN.md")
	}

	// Clean markdown block if present
	content := utils.CleanCodeBlock(response)

	if err := os.WriteFile(planPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write PLAN.md: %w", err)
	}

	// 5. Output
	fmt.Printf("\n✅ Plan generated successfully at: %s\n", planPath)
	s.Logger.Info("plan generated", "path", planPath)

	return nil
}
