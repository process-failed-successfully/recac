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

	// TPM
	resp, _ := agent.Send(context.Background(), "TPM Agent: Decompose the spec")
	if !strings.Contains(resp, "ID:[PRIMES]") {
		t.Errorf("Expected TPM response for PRIMES, got: %s", resp)
	}

	// Initializer
	resp, _ = agent.Send(context.Background(), "Initializer Agent: Identify all features")
	if !strings.Contains(resp, "agent-bridge import") {
		t.Errorf("Expected Initializer response, got: %s", resp)
	}

	// Coding
	resp, _ = agent.Send(context.Background(), "ID:[PRIMES] create primes.py")
	if !strings.Contains(resp, "cat << 'EOF' > primes.py") {
		t.Errorf("Expected Coding response, got: %s", resp)
	}
}
