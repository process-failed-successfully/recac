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

	// Test Planning Phase Heuristic
	promptPlanning := "You are the Technical Program Manager for this project..."
	respPlanning, err := agent.Send(context.Background(), promptPlanning)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(respPlanning, "Implement Prime Number Generator") {
		t.Errorf("Response missing planning content, got: %s", respPlanning)
	}

	// Test Coding Phase Heuristic
	promptCoding := "Please implement the feature described in ID:[PRIMES] Implement Prime Number Generator"
	respCoding, err := agent.Send(context.Background(), promptCoding)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(respCoding, "primes.py") {
		t.Errorf("Response missing python script creation, got: %s", respCoding)
	}
	if !strings.Contains(respCoding, "```bash") {
		t.Errorf("Response missing bash block, got: %s", respCoding)
	}

	// Test Coding Phase Completion Heuristic
	promptCodingDone := "Please implement the feature described in ID:[PRIMES] Implement Prime Number Generator... python3 primes.py"
	respCodingDone, err := agent.Send(context.Background(), promptCodingDone)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(respCodingDone, "Task completed") {
		t.Errorf("Response missing completion message, got: %s", respCodingDone)
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
