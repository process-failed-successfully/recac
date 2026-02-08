package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent_Initializer_Uppercase(t *testing.T) {
	agent := NewMockAgent()
	prompt := "## YOUR ROLE - INITIALIZER AGENT\n\nSome instructions..."

	resp, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(resp, "git init") && !strings.Contains(resp, "agent-bridge import") {
		t.Errorf("Expected Initializer response (git init/import), got: %s", resp)
	}
}

func TestMockAgent_PrimeScenario(t *testing.T) {
	agent := NewMockAgent()
	prompt := "## YOUR ROLE - INITIALIZER AGENT\n\nTask: Implement prime number script..."

	resp, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(resp, "Prime Number Generator") {
		t.Errorf("Expected Prime Number Generator in response, got: %s", resp)
	}
}

func TestMockAgent_TPM_Heuristic(t *testing.T) {
	agent := NewMockAgent()

	// 1. Positive case: Prompt with the new header
	prompt := "## YOUR ROLE - TECHNICAL PROGRAM MANAGER\n\nSome instructions..."
	resp, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(resp, `"type": "Task"`) {
		t.Errorf("Expected TPM response (JSON plan), got: %s", resp)
	}

	// 2. Negative case: Prompt mentioning TPM but without header (Simulating Coding Agent history)
	// This should NOT trigger TPM logic. It should hit fallback.
	prompt2 := "## YOUR ROLE - CODING AGENT\n\nHistory: The Technical Program Manager said..."
	resp2, err := agent.Send(context.Background(), prompt2)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if strings.Contains(resp2, `"type": "Task"`) {
		t.Errorf("Coding Agent prompt incorrectly triggered TPM response: %s", resp2)
	}
	if !strings.Contains(resp2, "Mock agent response") {
		t.Errorf("Expected fallback response, got: %s", resp2)
	}

	// 3. Coding Agent Scenario with TPM in history
	// This should trigger Coding Agent logic, NOT TPM logic.
	prompt3 := "## YOUR ROLE - CODING AGENT\n\nTask: [PRIMES] implementation.\nHistory: Technical Program Manager provided the plan."
	resp3, err := agent.Send(context.Background(), prompt3)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if strings.Contains(resp3, `"type": "Task"`) {
		t.Errorf("Coding Agent prompt with [PRIMES] incorrectly triggered TPM response due to history: %s", resp3)
	}
	if !strings.Contains(resp3, "I will implement the prime number script") {
		t.Errorf("Expected Coding Agent implementation response, got: %s", resp3)
	}
}

func TestMockAgent_QA_Heuristic(t *testing.T) {
	agent := NewMockAgent()
	prompt := "## YOUR ROLE - QA AGENT\n\nSome instructions..."

	resp, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(resp, "agent-bridge signal QA_PASSED") {
		t.Errorf("Expected QA Agent response with signal command, got: %s", resp)
	}
}
