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

func TestMockAgent_Primes(t *testing.T) {
	agent := NewMockAgent()

	// Test Step 1: Ticket Generation
	planPrompt := "ID:[PRIMES] Prime Number Script"
	planResp, err := agent.Send(context.Background(), planPrompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(planResp, "[PRIMES] Create Prime Number Script") {
		t.Errorf("Expected plan JSON, got: %s", planResp)
	}

	// Test Step 2: Implementation
	// The prompt needs to trigger the condition: strings.Contains(prompt, "Create a python script named 'primes.py'")
	// AND also contain ID:[PRIMES] or Prime Number Script to enter the block.
	implPrompt := "ID:[PRIMES] Please Create a python script named 'primes.py'. It MUST be python."
	implResp, err := agent.Send(context.Background(), implPrompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(implResp, "cat << 'EOF' > primes.py") {
		t.Errorf("Expected bash block for primes.py, got: %s", implResp)
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
