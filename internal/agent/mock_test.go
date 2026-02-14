package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent_PrimesHeuristic(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()
	prompt := "Please ID:[PRIMES] Implement Prime Number Generator"

	response, err := agent.Send(ctx, prompt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(response, "python3 primes.py") {
		t.Errorf("response missing python execution command")
	}
	if !strings.Contains(response, "git commit") {
		t.Errorf("response missing git commit command")
	}
	if !strings.Contains(response, "agent-bridge signal PROJECT_SIGNED_OFF true --privileged") {
		t.Errorf("response missing completion signal")
	}
}

func TestMockAgent_PlanningHeuristic(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()
	prompt := "You are a Technical Program Manager (TPM)"

	response, err := agent.Send(ctx, prompt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(response, "PRIMES-1") {
		t.Errorf("response missing mock ticket ID")
	}
}

func TestMockAgent_DefaultResponse(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()
	prompt := "Hello world"

	response, err := agent.Send(ctx, prompt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(response, "Mock agent response") {
		t.Errorf("unexpected default response: %s", response)
	}
}
