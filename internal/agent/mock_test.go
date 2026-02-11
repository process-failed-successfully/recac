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

	// 1. TPM Role (Jira Planning)
	tpmPrompt := "You are an expert Technical Program Manager (TPM).\nOutput purely JSON in the following format...\n### ID:[PRIMES] Prime Number Script\nImplement primes.py ..."

	t.Run("TPM Role - Primes", func(t *testing.T) {
		resp, err := agent.Send(ctx, tpmPrompt)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Should return JSON, not bash
		if strings.Contains(resp, "```bash") {
			t.Errorf("TPM role should not return bash script")
		}
		if !strings.Contains(resp, "ID:[PRIMES]") {
			t.Errorf("TPM role should return ID:[PRIMES] in JSON")
		}
		if !strings.Contains(resp, "\"type\": \"Task\"") {
			t.Errorf("TPM role should create a Task as requested")
		}
	})

	// 2. Coding Role (Implementation)
	codingPrompt := "Your role - Coding Agent.\nImplement primes.py ..."

	t.Run("Coding Role - Primes", func(t *testing.T) {
		resp, err := agent.Send(ctx, codingPrompt)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Should return bash script
		if !strings.Contains(resp, "```bash") {
			t.Errorf("Coding role should return bash script")
		}
		if !strings.Contains(resp, "primes.py") {
			t.Errorf("Coding role should implement primes.py")
		}
	})
}
