package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent_Fallback(t *testing.T) {
	agent := NewMockAgent()

	prompt := "This is a random prompt that should trigger fallback"
	response, err := agent.Send(context.Background(), prompt)

	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(response, "Mock agent response") {
		t.Errorf("Response missing prefix, got: %s", response)
	}

	if !strings.Contains(response, "echo \"Mock Agent Default Response\"") {
		t.Errorf("Response missing fallback command, got: %s", response)
	}
}

func TestMockAgent_TPM(t *testing.T) {
	agent := NewMockAgent()

	// Test Generic TPM
	prompt := "You are a Technical Program Manager. Please plan."
	response, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(response, "TASK-1") {
		t.Errorf("Expected JSON with TASK-1, got: %s", response)
	}

	// Test Primes TPM
	promptPrimes := "You are a Technical Program Manager. Plan the [PRIMES] scenario."
	responsePrimes, err := agent.Send(context.Background(), promptPrimes)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(responsePrimes, "Implement Primes") {
		t.Errorf("Expected Primes task, got: %s", responsePrimes)
	}
}

func TestMockAgent_Architect(t *testing.T) {
	agent := NewMockAgent()

	prompt := "You are the Lead Software Architect."
	response, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(response, "\"features\":") {
		t.Errorf("Expected JSON with features, got: %s", response)
	}
}

func TestMockAgent_Coding(t *testing.T) {
	agent := NewMockAgent()

	prompt := "Please implement primes.py"
	response, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(response, "def is_prime(n):") {
		t.Errorf("Expected python code for primes, got: %s", response)
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
