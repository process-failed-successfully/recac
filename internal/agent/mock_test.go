package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent_TPM(t *testing.T) {
	agent := NewMockAgent()
	prompt := "You are a Technical Program Manager (TPM)"
	response, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(response, "Epic: Implement Core Features") {
		t.Errorf("Expected TPM response, got: %s", response)
	}
}

func TestMockAgent_TPM_Primes(t *testing.T) {
	agent := NewMockAgent()
	prompt := "You are a Technical Program Manager (TPM)... [PRIMES]"
	response, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(response, "Task") || !strings.Contains(response, "Prime Number Script") {
		t.Errorf("Expected TPM response to contain Task and Prime Number Script, got: %s", response)
	}
}

func TestMockAgent_Coding(t *testing.T) {
	agent := NewMockAgent()
	prompt := "Implement the features" // Generic prompt
	response, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(response, "agent-bridge feature set") {
		t.Errorf("Expected coding response with feature update, got: %s", response)
	}
}

func TestMockAgent_Coding_Primes(t *testing.T) {
	agent := NewMockAgent()
	prompt := "Implement primes.py"
	response, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(response, "python3 primes.py") {
		t.Errorf("Expected coding response to run primes.py, got: %s", response)
	}
	if !strings.Contains(response, "import json") {
		t.Errorf("Expected python implementation, got: %s", response)
	}
}

func TestMockAgent_QA(t *testing.T) {
	agent := NewMockAgent()
	prompt := "YOUR ROLE - QA AGENT"
	response, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(response, "QA_PASSED") {
		t.Errorf("Expected QA signal, got: %s", response)
	}
}

func TestMockAgent_Coding_Primes_Fallback_Fix(t *testing.T) {
	agent := NewMockAgent()
	// Simulate the prompt that caused the failure: contains ticket summary but not explicit file name or ID
	prompt := "Implement feature: Create Prime Number Script"
	response, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(response, "import json") {
		t.Errorf("Expected full python implementation (import json), got: %s", response)
	}
	if !strings.Contains(response, "primes = []") {
		t.Errorf("Expected full python implementation (primes = []), got: %s", response)
	}
}
