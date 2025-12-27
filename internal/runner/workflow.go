package runner

import (
	"context"
	"fmt"
	"recac/internal/agent"
	"recac/internal/pkg/git"
)

// Workflow orchestrates the development cycle.
type Workflow struct {
	PlannerAgent agent.Agent // For decomposing spec
	ManagerAgent agent.Agent // For high-level review
	Features     []Feature
	GitOps       git.GitOps  // For git operations (push, create PR)
}

// NewWorkflow creates a new workflow.
func NewWorkflow(planner, manager agent.Agent) *Workflow {
	return &Workflow{
		PlannerAgent: planner,
		ManagerAgent: manager,
		Features:     []Feature{},
		GitOps:       git.DefaultGitOps,
	}
}

// NewWorkflowWithGitOps creates a new workflow with custom GitOps (for testing).
func NewWorkflowWithGitOps(planner, manager agent.Agent, gitOps git.GitOps) *Workflow {
	return &Workflow{
		PlannerAgent: planner,
		ManagerAgent: manager,
		Features:     []Feature{},
		GitOps:       gitOps,
	}
}

// RunCycle executes a development cycle: QA -> Manager Review.
func (w *Workflow) RunCycle(ctx context.Context) (string, error) {
	// 1. QA Phase
	qaReport := RunQA(w.Features)
	fmt.Println("QA Phase Complete.")
	fmt.Println(qaReport.String())

	// 2. Manager Review Phase
	fmt.Println("Manager Review starting...")
	prompt := fmt.Sprintf(`
You are the Engineering Manager.
Here is the current QA Report:
%s

Please provide strategic feedback on what to focus on next.
`, qaReport.String())

	feedback, err := w.ManagerAgent.Send(ctx, prompt)
	if err != nil {
		return "", fmt.Errorf("manager review failed: %w", err)
	}

	return feedback, nil
}

// FinishFeature handles the end of a feature: push and PR creation.
func (w *Workflow) FinishFeature(ctx context.Context, featureName string) error {
	branch := fmt.Sprintf("feature/%s", featureName)
	
	// 1. Push
	if err := w.GitOps.Push(branch); err != nil {
		return fmt.Errorf("failed to push branch: %w", err)
	}
	
	// 2. Generate PR Description using Agent
	prompt := fmt.Sprintf("Generate a PR description for the feature: %s", featureName)
	description, err := w.ManagerAgent.Send(ctx, prompt)
	if err != nil {
		description = "Automated PR for " + featureName
	}
	
	// 3. Create PR
	title := "Implement " + featureName
	if err := w.GitOps.CreatePR(branch, title, description); err != nil {
		return fmt.Errorf("failed to create PR: %w", err)
	}
	
	return nil
}