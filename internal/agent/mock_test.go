package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent_Default(t *testing.T) {
	agent := NewMockAgent()
	prompt := "This is a generic prompt that doesn't trigger any role"
	response, err := agent.Send(context.Background(), prompt)

	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(response, "Mock agent response") {
		t.Errorf("Expected generic mock response, got: %s", response)
	}
}

func TestMockAgent_Initializer(t *testing.T) {
	agent := NewMockAgent()
	// Test both legacy and new prompt formats
	prompts := []string{
		"You are an Initializer Agent",
		"## YOUR ROLE - INITIALIZER AGENT",
	}

	for _, prompt := range prompts {
		response, err := agent.Send(context.Background(), prompt)
		if err != nil {
			t.Fatalf("Send failed: %v", err)
		}

		if !strings.Contains(response, "agent-bridge import") {
			t.Errorf("Expected agent-bridge import command for Initializer role (prompt: %s), got: %s", prompt, response)
		}
	}
}

func TestMockAgent_TPM(t *testing.T) {
	agent := NewMockAgent()
	prompt := "You are an expert Technical Program Manager (TPM)"
	response, err := agent.Send(context.Background(), prompt)

	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.HasPrefix(strings.TrimSpace(response), "[") || !strings.HasSuffix(strings.TrimSpace(response), "]") {
		t.Errorf("Expected JSON array for TPM role, got: %s", response)
	}
	if !strings.Contains(response, "\"title\": \"ID:[PRIMES]") {
		t.Errorf("Expected prime implementation task in TPM response, got: %s", response)
	}
}

func TestMockAgent_Coding_Prime(t *testing.T) {
	agent := NewMockAgent()
	prompt := "Implement a function to check if a number is PRIME"
	response, err := agent.Send(context.Background(), prompt)

	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(response, "def is_prime(n):") {
		t.Errorf("Expected Python code for Coding role (PRIME), got: %s", response)
	}
}

func TestMockAgent_Coding_Python(t *testing.T) {
	agent := NewMockAgent()
	prompt := "Write a python script to calculate numbers"
	response, err := agent.Send(context.Background(), prompt)

	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(response, "def is_prime(n):") {
		t.Errorf("Expected Python code for Coding role (PYTHON), got: %s", response)
	}
}

func TestMockAgent_QA(t *testing.T) {
	agent := NewMockAgent()
	prompt := "You are a QA agent. Review the code."
	response, err := agent.Send(context.Background(), prompt)

	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if strings.TrimSpace(response) != "QA_PASSED" {
		t.Errorf("Expected QA_PASSED signal, got: %s", response)
	}
}

func TestMockAgent_Verify(t *testing.T) {
	agent := NewMockAgent()
	prompt := "Please verify the implementation."
	response, err := agent.Send(context.Background(), prompt)

	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if strings.TrimSpace(response) != "QA_PASSED" {
		t.Errorf("Expected QA_PASSED signal for verify, got: %s", response)
	}
}
