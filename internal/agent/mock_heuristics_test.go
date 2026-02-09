package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent_HeuristicCollision(t *testing.T) {
	agent := NewMockAgent()

	// Simulate a Coding Agent prompt that includes instructions about QA
	// This matches the structure of internal/agent/prompts/templates/coding_agent.md
	// Crucially, it must contain "QA" in uppercase to trigger the bug.
	prompt := `
## YOUR ROLE - CODING AGENT

Your task is to implement the feature: [PRIMES]

Instructions:
1. Implement the code.
2. Quality Assurance: Run agent-bridge qa (Triggers QA Agent).
`

	response, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// We expect the agent to generate python code (Heuristic 5)
	// We do NOT expect it to signal QA_PASSED (Heuristic 4)
	if strings.Contains(response, "agent-bridge signal QA_PASSED true") {
		t.Errorf("FAIL: MockAgent incorrectly triggered QA Heuristic instead of Coding Heuristic. Response:\n%s", response)
	}

	if !strings.Contains(response, "cat << 'EOF' > primes.py") {
		t.Errorf("FAIL: MockAgent failed to trigger Coding Heuristic (create primes.py). Response:\n%s", response)
	}
}

func TestMockAgent_QAHeuristic(t *testing.T) {
	agent := NewMockAgent()

	// Simulate a QA Agent prompt
	prompt := `
## YOUR ROLE - QA AGENT

Your job is to verify the project.
`
	response, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// QA Agent should return QA_PASSED
	if !strings.Contains(response, "agent-bridge signal QA_PASSED true") {
		t.Errorf("FAIL: MockAgent failed to trigger QA Heuristic for valid QA prompt. Response:\n%s", response)
	}
}

func TestMockAgent_ManagerHeuristic(t *testing.T) {
	agent := NewMockAgent()

	// Simulate a Project Manager prompt
	prompt := `
## YOUR ROLE - PROJECT MANAGER

Review the QA Report.
`
	response, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Manager should return PROJECT_SIGNED_OFF, possibly with --privileged
	if !strings.Contains(response, "PROJECT_SIGNED_OFF true") {
		t.Errorf("FAIL: MockAgent failed to trigger Manager Heuristic for valid Manager prompt. Response:\n%s", response)
	}
}
