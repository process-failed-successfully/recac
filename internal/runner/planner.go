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

// GeneratePlanOnly generates a detailed implementation plan and writes it to PLAN.md.
func GeneratePlanOnly(ctx context.Context, s *Session) error {
	// 1. Generate Context
	opts := utils.ContextOptions{
		Roots:   []string{s.Workspace},
		MaxSize: 200 * 1024,
		Tree:    true,
	}
	codebaseContext, err := utils.GenerateCodebaseContext(opts)
	if err != nil {
		return fmt.Errorf("failed to generate codebase context: %w", err)
	}

	// 2. Read Spec
	spec, err := s.ReadSpec()
	if err != nil {
		return fmt.Errorf("failed to read spec: %w", err)
	}

	// 3. Prepare Prompt
	prompt, err := prompts.GetPrompt(prompts.PlanOnly, map[string]string{
		"spec":             spec,
		"codebase_context": codebaseContext,
	})
	if err != nil {
		return fmt.Errorf("failed to load plan_only prompt: %w", err)
	}

	// 4. Call Agent
	fmt.Println("Generating implementation plan...")
	response, err := s.Agent.Send(ctx, prompt)
	if err != nil {
		return fmt.Errorf("agent failed to generate plan: %w", err)
	}

	// 5. Write PLAN.md
	planPath := filepath.Join(s.Workspace, "PLAN.md")
	if err := os.WriteFile(planPath, []byte(response), 0644); err != nil {
		return fmt.Errorf("failed to write PLAN.md: %w", err)
	}

	fmt.Printf("Plan generated successfully: %s\n", planPath)
	return nil
}
