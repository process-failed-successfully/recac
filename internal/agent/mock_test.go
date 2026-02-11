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

func TestMockAgent_TPM(t *testing.T) {
	agent := NewMockAgent()
	prompts := []string{
		"You are an expert Technical Program Manager (TPM)",
		"## YOUR ROLE - PROJECT MANAGER",
	}

	for _, prompt := range prompts {
		response, err := agent.Send(context.Background(), prompt)
		if err != nil {
			t.Fatalf("Send failed for prompt '%s': %v", prompt, err)
		}

		if !strings.HasPrefix(strings.TrimSpace(response), "[") || !strings.HasSuffix(strings.TrimSpace(response), "]") {
			t.Errorf("Expected JSON array for TPM role with prompt '%s', got: %s", prompt, response)
		}
		if !strings.Contains(response, "\"title\": \"ID:[PRIMES]") {
			t.Errorf("Expected prime implementation task in TPM response with prompt '%s', got: %s", prompt, response)
		}
	}
}

func TestMockAgent_Initializer(t *testing.T) {
	agent := NewMockAgent()
	prompts := []string{
		"You are an Initializer Agent",
		"## YOUR ROLE - INITIALIZER AGENT",
	}

	for _, prompt := range prompts {
		response, err := agent.Send(context.Background(), prompt)
		if err != nil {
			t.Fatalf("Send failed for prompt '%s': %v", prompt, err)
		}

		if !strings.Contains(response, "agent-bridge import") {
			t.Errorf("Expected agent-bridge import for Initializer role with prompt '%s', got: %s", prompt, response)
		}
	}
}

func TestMockAgent_Coding(t *testing.T) {
	agent := NewMockAgent()
	prompts := []string{
		"Implement a function to check if a number is PRIME",
		"Write a python script to calculate numbers",
		"You are a Coding Agent",
	}

	for _, prompt := range prompts {
		response, err := agent.Send(context.Background(), prompt)
		if err != nil {
			t.Fatalf("Send failed for prompt '%s': %v", prompt, err)
		}

		if !strings.Contains(response, "def is_prime(n):") {
			t.Errorf("Expected Python code for Coding role with prompt '%s', got: %s", prompt, response)
		}
	}
}

func TestMockAgent_QA(t *testing.T) {
	agent := NewMockAgent()
	prompts := []string{
		"You are a QA agent. Review the code.",
		"Please verify the implementation.",
		"## YOUR ROLE - QA AGENT",
	}

	for _, prompt := range prompts {
		response, err := agent.Send(context.Background(), prompt)
		if err != nil {
			t.Fatalf("Send failed for prompt '%s': %v", prompt, err)
		}

		if strings.TrimSpace(response) != "QA_PASSED" {
			t.Errorf("Expected QA_PASSED signal for prompt '%s', got: %s", prompt, response)
		}
	}
}
