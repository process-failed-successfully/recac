package runner

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"recac/internal/agent/prompts"
)

// GeneratePlanOnly generates a plan for the project and exits.
func (s *Session) GeneratePlanOnly(ctx context.Context) error {
	s.Logger.Info("Generating plan only...")

	// 1. Read Spec
	spec, err := s.ReadSpec()
	if err != nil {
		return fmt.Errorf("failed to read spec: %w", err)
	}

	// 2. Prepare Prompt
	promptVars := map[string]string{
		"spec": spec,
	}

	prompt, err := prompts.GetPrompt(prompts.PlanOnly, promptVars)
	if err != nil {
		return fmt.Errorf("failed to get plan_only prompt: %w", err)
	}

	// 3. Send to Agent
	s.Logger.Info("Sending plan generation prompt to agent")
	fmt.Println("Generating PLAN.md...")

	var response string
	if s.StreamOutput {
		fmt.Print("Agent Response: ")
		response, err = s.Agent.SendStream(ctx, prompt, func(chunk string) {
			fmt.Print(chunk)
		})
		fmt.Println() // Newline after stream
	} else {
		response, err = s.Agent.Send(ctx, prompt)
	}

	if err != nil {
		return fmt.Errorf("agent failed to generate plan: %w", err)
	}

	// 4. Save PLAN.md
	planPath := filepath.Join(s.Workspace, "PLAN.md")
	if err := os.WriteFile(planPath, []byte(response), 0644); err != nil {
		return fmt.Errorf("failed to write PLAN.md: %w", err)
	}

	s.Logger.Info("PLAN.md generated successfully", "path", planPath)
	fmt.Printf("PLAN.md generated successfully at %s\n", planPath)

	// Sync to DB if available
	if s.DBStore != nil {
		if err := s.DBStore.SaveObservation(s.Project, "Planner", "Generated PLAN.md"); err != nil {
			s.Logger.Warn("failed to save planner observation", "error", err)
		}
	}

	return nil
}
