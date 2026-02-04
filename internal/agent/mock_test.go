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

func TestMockAgent_E2E_Scenarios(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	// 1. Initializer Scenario
	respInit, err := agent.Send(ctx, "You are the INITIALIZER AGENT.")
	if err != nil {
		t.Fatalf("Initializer prompt failed: %v", err)
	}
	if !strings.Contains(respInit, "agent-bridge import") {
		t.Error("Initializer response missing 'agent-bridge import'")
	}
	if !strings.Contains(respInit, "req-primes-py-exists") {
		t.Error("Initializer response missing feature id")
	}

	// 2. Primes Implementation Scenario
	respPrimes, err := agent.Send(ctx, "Please write primes.py")
	if err != nil {
		t.Fatalf("Primes prompt failed: %v", err)
	}
	if !strings.Contains(respPrimes, "cat << 'EOF' > primes.py") {
		t.Error("Primes response missing file creation")
	}
	if !strings.Contains(respPrimes, "agent-bridge feature set req-primes-py-exists passed") {
		t.Error("Primes response missing feature update signal")
	}
}
