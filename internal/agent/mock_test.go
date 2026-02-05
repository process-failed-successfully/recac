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

func TestMockAgent_RoleDetection(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	// 1. Test Manager Role Detection (Positive)
	managerPrompt := "## YOUR ROLE - PROJECT MANAGER\n\nReview the QA report..."
	resp, err := agent.Send(ctx, managerPrompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(resp, "PROJECT_SIGNED_OFF") {
		t.Error("Expected Manager response for 'ROLE - PROJECT MANAGER', got generic response")
	}

	// 2. Test Coding Agent Confusion (Negative)
	// This prompt mentions 'Project Manager' in the body but defines 'ROLE - CODING AGENT'
	codingPrompt := `## YOUR ROLE - CODING AGENT

### COMMUNICATE WITH MANAGER
You have a Project Manager who reviews your work periodically.
Triggers Manager Review: agent-bridge manager
`
	resp, err = agent.Send(ctx, codingPrompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Should NOT return the Manager response
	if strings.Contains(resp, "PROJECT_SIGNED_OFF") {
		t.Error("Coding Agent prompt incorrectly triggered Manager response because of loose keyword matching")
	}

	// Should match default generic response (or Primes logic if triggered, but here generic)
	if !strings.Contains(resp, "Mock agent response") {
		t.Errorf("Expected generic mock response, got: %s", resp)
	}
}

func TestMockAgent_InitializerDetection(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	// 1. Positive Case
	initPrompt := "## YOUR ROLE - INITIALIZER AGENT\n\nCreate feature_list.json..."
	resp, err := agent.Send(ctx, initPrompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(resp, "agent-bridge import") {
		t.Error("Expected Initializer response for 'ROLE - INITIALIZER AGENT'")
	}

	// 2. Negative Case (Coding Agent with feature_list.json)
	codingPrompt := `## YOUR ROLE - CODING AGENT

Reading feature_list.json to understand tasks...
`
	resp, err = agent.Send(ctx, codingPrompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Should NOT return the Initializer response
	if strings.Contains(resp, "agent-bridge import") {
		t.Error("Coding Agent prompt incorrectly triggered Initializer response")
	}
}
