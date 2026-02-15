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

func TestMockAgent_Heuristics(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	// 1. Test TPM/Initializer
	tpmPrompt := "You are an expert Technical Program Manager (TPM). Please create a plan for ID:[PRIMES] Prime Number Script."
	resp, err := agent.Send(ctx, tpmPrompt)
	if err != nil {
		t.Fatalf("TPM Send failed: %v", err)
	}
	if !strings.Contains(resp, "ID:[PRIMES]") {
		t.Errorf("TPM response missing ID:[PRIMES], got: %s", resp)
	}
	if !strings.Contains(resp, "[") || !strings.Contains(resp, "{") {
		t.Errorf("TPM response does not look like JSON, got: %s", resp)
	}

	// 2. Test Coding Agent
	codingPrompt := "You are a Coding Agent. Please write code for prime number generation."
	resp, err = agent.Send(ctx, codingPrompt)
	if err != nil {
		t.Fatalf("Coding Send failed: %v", err)
	}
	if !strings.Contains(resp, "primes.py") {
		t.Errorf("Coding response missing primes.py, got: %s", resp)
	}
	if strings.HasPrefix(resp, "[") {
		t.Errorf("Coding response looks like JSON (TPM heuristic leaked?), got: %s", resp)
	}

	// 3. Test QA Agent
	qaPrompt := "You are a QA Agent. Verify the work."
	resp, err = agent.Send(ctx, qaPrompt)
	if err != nil {
		t.Fatalf("QA Send failed: %v", err)
	}
	if !strings.Contains(resp, "QA_PASSED") {
		t.Errorf("QA response missing QA_PASSED, got: %s", resp)
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
