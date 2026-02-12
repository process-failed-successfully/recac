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

func TestMockAgent_Primes(t *testing.T) {
	agent := NewMockAgent()

	// Test Planning Phase
	planPrompt := "create exactly ONE ticket for PRIMES"
	planResponse, err := agent.Send(context.Background(), planPrompt)
	if err != nil {
		t.Fatalf("Plan Send failed: %v", err)
	}
	if !strings.Contains(planResponse, "\"id\": \"PRIMES\"") {
		t.Errorf("Plan response missing PRIMES ticket, got: %s", planResponse)
	}

	// Test Implementation Phase
	implPrompt := "Implement a python script named 'primes.py'"
	implResponse, err := agent.Send(context.Background(), implPrompt)
	if err != nil {
		t.Fatalf("Impl Send failed: %v", err)
	}
	if !strings.Contains(implResponse, "cat << 'EOF' > primes.py") {
		t.Errorf("Impl response missing bash script, got: %s", implResponse)
	}
	if !strings.Contains(implResponse, "Task Completed") {
		t.Errorf("Impl response missing completion signal, got: %s", implResponse)
	}
}
