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

func TestMockAgent_Scenario_Primes_Manager(t *testing.T) {
	agent := NewMockAgent()
	prompt := "## ROLE: Project Manager\n\nReview the implementation of [PRIMES] feature."

	response, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Should NOT implement code (Coding Agent heuristic #5)
	if strings.Contains(response, "def is_prime(n):") {
		t.Errorf("Manager agent should not implement code! Got implementation snippet.")
	}

	// Should approve or signal completion (Manager heuristic #4)
	if !strings.Contains(response, "agent-bridge feature set PRIMES --status done --passes true") {
		t.Errorf("Manager agent should approve feature! Got: %s", response)
	}
}

func TestMockAgent_Scenario_Primes_QA(t *testing.T) {
	agent := NewMockAgent()
	prompt := "## ROLE: QA Agent\n\nVerify the implementation of [PRIMES] feature."

	response, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Should NOT implement code
	if strings.Contains(response, "def is_prime(n):") {
		t.Errorf("QA agent should not implement code! Got implementation snippet.")
	}

	// Should pass QA
	if !strings.Contains(response, "agent-bridge feature set PRIMES --status done --passes true") {
		t.Errorf("QA agent should pass feature! Got: %s", response)
	}
}
