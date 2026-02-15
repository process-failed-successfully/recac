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
	ctx := context.Background()
	prompt := "Please implement the prime number generator task in python"

	// Call 1: Expect script creation
	resp1, err := agent.Send(ctx, prompt)
	if err != nil {
		t.Fatalf("Call 1 failed: %v", err)
	}
	if !strings.Contains(resp1, "cat << 'EOF' > primes.py") {
		t.Errorf("Expected script creation in response 1, got: %s", resp1)
	}
	if !strings.Contains(resp1, "python3 primes.py") {
		t.Errorf("Expected execution in response 1, got: %s", resp1)
	}

	// Call 2: Expect commit and completion
	resp2, err := agent.Send(ctx, prompt)
	if err != nil {
		t.Fatalf("Call 2 failed: %v", err)
	}
	if !strings.Contains(resp2, "git commit") {
		t.Errorf("Expected git commit in response 2, got: %s", resp2)
	}
	if !strings.Contains(resp2, "agent-bridge signal COMPLETED true") {
		t.Errorf("Expected completion signal in response 2, got: %s", resp2)
	}
}

func TestMockAgent_TPM(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()
	prompt := "You are the Technical Program Manager. Output the plan in json."

	resp, err := agent.Send(ctx, prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(resp, `"id": "TASK-1"`) {
		t.Errorf("Expected JSON plan, got: %s", resp)
	}
}
