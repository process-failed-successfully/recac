package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent(t *testing.T) {
	agent := NewMockAgent()

	prompt := "This is a test prompt that is long enough to be truncated"
	response, err := agent.Send(context.Background(), prompt)

	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(response, "Mock agent response") {
		t.Errorf("Response missing prefix, got: %s", response)
	}

	if !strings.Contains(response, "I received your prompt") {
		t.Errorf("Response missing body, got: %s", response)
	}
}

func TestTruncateString(t *testing.T) {
	s := "hello world"
	if truncateString(s, 5) != "hello" {
		t.Errorf("Expected 'hello', got '%s'", truncateString(s, 5))
	}
	if truncateString(s, 20) != "hello world" {
		t.Errorf("Expected 'hello world', got '%s'", truncateString(s, 20))
	}
}

func TestMockAgent_Heuristics(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	// 1. Test Planner Heuristic (Should Match)
	plannerPrompt := "You are a Technical Program Manager. Please generate tickets."
	resp, err := agent.Send(ctx, plannerPrompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(resp, "ID:[PRIMES]") {
		t.Errorf("Expected JSON plan for Planner prompt, got: %s", resp)
	}

	// 2. Test Coding Agent Avoidance (Should NOT Match Planner)
	// Even if "app_spec.txt" is present, "YOUR ROLE - CODING AGENT" should prevent planner match
	codingPrompt := "## YOUR ROLE - CODING AGENT\n\nRead cat app_spec.txt"
	resp, err = agent.Send(ctx, codingPrompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if strings.Contains(resp, "ID:[PRIMES]") && strings.HasPrefix(strings.TrimSpace(resp), "[") {
		t.Error("MockAgent incorrectly returned JSON plan for Coding Agent prompt (false positive)")
	}

	// Should fall back to default response (since [PRIMES] is not in prompt)
	if !strings.Contains(resp, "Mock agent response") && !strings.Contains(resp, "Mock Agent is alive") {
		t.Logf("Response was: %s", resp)
	}
}
