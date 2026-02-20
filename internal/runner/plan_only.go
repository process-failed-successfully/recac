package runner

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"recac/internal/agent/prompts"
)

// GeneratePlanOnly generates a detailed implementation plan without executing it.
func (s *Session) GeneratePlanOnly(ctx context.Context) error {
	s.Logger.Info("Running in PLAN ONLY mode")

	// 1. Read Spec
	spec, err := s.ReadSpec()
	if err != nil {
		return fmt.Errorf("failed to read spec: %w", err)
	}

	// 2. Construct Prompt
	prompt, err := prompts.GetPrompt(prompts.PlanOnly, map[string]string{
		"spec": spec,
	})
	if err != nil {
		return fmt.Errorf("failed to load plan_only prompt: %w", err)
	}

	s.Logger.Info("Sending planning prompt to agent...", "model", s.AgentModel)

	// 3. Send to Agent
	var response string
	if s.StreamOutput {
		fmt.Print("Agent Planning Response: ")
		response, err = s.Agent.SendStream(ctx, prompt, func(chunk string) {
			fmt.Print(chunk)
		})
		fmt.Println()
	} else {
		response, err = s.Agent.Send(ctx, prompt)
	}

	if err != nil {
		return fmt.Errorf("agent planning failed: %w", err)
	}

	// 4. Save Plan
	planPath := filepath.Join(s.Workspace, "PLAN.md")
	if err := os.WriteFile(planPath, []byte(response), 0644); err != nil {
		return fmt.Errorf("failed to save PLAN.md: %w", err)
	}

	s.Logger.Info("Plan generated successfully", "path", planPath)
	fmt.Printf("\n[PLAN ONLY] Implementation plan saved to %s\n", planPath)

	return nil
}
