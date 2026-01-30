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

func TestMockAgent_PrimePythonScenario(t *testing.T) {
	agent := NewMockAgent()

	// 1. Ticket Generation
	tpmPrompt := "You are an expert Technical Program Manager... please generate tickets in JSON..."
	resp, err := agent.Send(context.Background(), tpmPrompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(resp, "[") || !strings.Contains(resp, "ID:[PRIMES]") {
		t.Errorf("Expected JSON ticket response, got: %s", resp)
	}

	// 2. Implementation
	devPrompt := "Implement the Prime Number Generator... files: primes.py"
	resp, err = agent.Send(context.Background(), devPrompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(resp, "```bash") || !strings.Contains(resp, "primes.py") {
		t.Errorf("Expected bash script response, got: %s", resp)
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
