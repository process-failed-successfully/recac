package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent_Heuristics(t *testing.T) {
	agent := NewMockAgent()

	// 1. TPM / Jira Generation
	tpmPrompt := "You are an expert Technical Program Manager (TPM) with deep experience in agile software development. Create exactly ONE ticket."
	resp, err := agent.Send(context.Background(), tpmPrompt)
	if err != nil {
		t.Fatalf("TPM Send failed: %v", err)
	}
	if !strings.Contains(resp, "[") || !strings.Contains(resp, "{") {
		t.Errorf("TPM response expected JSON array/object, got: %s", resp)
	}
	if !strings.Contains(resp, "PRIMES") {
		t.Errorf("TPM response expected ID:PRIMES, got: %s", resp)
	}

	// 2. Initializer Agent
	initPrompt := "You are the Initializer Agent."
	resp, err = agent.Send(context.Background(), initPrompt)
	if err != nil {
		t.Fatalf("Initializer Send failed: %v", err)
	}
	if !strings.Contains(resp, "agent-bridge import") {
		t.Errorf("Initializer response expected agent-bridge import, got: %s", resp)
	}

	// 3. Coding Agent
	codingPrompt := "Write a python script primes.py that calculates primes."
	resp, err = agent.Send(context.Background(), codingPrompt)
	if err != nil {
		t.Fatalf("Coding Send failed: %v", err)
	}
	if !strings.Contains(resp, "def is_prime") {
		t.Errorf("Coding response expected python code, got: %s", resp)
	}
    if !strings.Contains(resp, "range(10000)") {
        t.Errorf("Coding response expected range(10000), got: %s", resp)
    }

	// 4. QA Agent
	qaPrompt := "QA Agent: Approve or Reject the changes."
	resp, err = agent.Send(context.Background(), qaPrompt)
	if err != nil {
		t.Fatalf("QA Send failed: %v", err)
	}
	if !strings.Contains(resp, "agent-bridge signal QA_PASSED true") {
		t.Errorf("QA response expected QA_PASSED signal, got: %s", resp)
	}
}
