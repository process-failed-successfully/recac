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

	// Test Ticket Generation Heuristic
	genPrompt := "ID:[PRIMES] Create a SINGLE Ticket (Task) for this work."
	genResp, err := agent.Send(context.Background(), genPrompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(genResp, "ID:[PRIMES] Prime Number Script") {
		t.Errorf("Expected ticket generation JSON, got: %s", genResp)
	}

	// Test Execution Heuristic
	execPrompt := "Implement a python script named 'primes.py'"
	execResp, err := agent.Send(context.Background(), execPrompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(execResp, "cat << 'EOF' > primes.py") {
		t.Errorf("Expected bash script execution, got: %s", execResp)
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
