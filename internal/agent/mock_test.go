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

func TestMockAgent_Coding_Primes(t *testing.T) {
	agent := NewMockAgent()
	// Simulate a coding agent prompt that includes "feature_list.json" which previously triggered the Initializer heuristic
	prompt := `## YOUR ROLE - CODING AGENT
	...
	cat feature_list.json | head -50
	...
	[PRIMES] Create Prime Number Script
	`

	response, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Should NOT return the initializer script (agent-bridge import)
	if strings.Contains(response, "agent-bridge import") {
		t.Error("Mock Agent incorrectly returned Initializer script instead of Coding implementation")
	}

	// Should return the python implementation
	if !strings.Contains(response, "cat << 'EOF' > primes.py") {
		t.Error("Mock Agent failed to return python implementation")
	}
}

func TestMockAgent_Initializer(t *testing.T) {
	agent := NewMockAgent()
	prompt := `[PRIMES] Initialize the project`

	response, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(response, "agent-bridge import") {
		t.Error("Mock Agent failed to return Initializer script")
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
