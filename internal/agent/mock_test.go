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

	// Verify no bash block (prevents infinite loops)
	if strings.Contains(response, "```bash") {
		t.Errorf("Response should NOT contain bash block for default prompt")
	}
}

func TestMockAgent_Primes_Planning(t *testing.T) {
	agent := NewMockAgent()

	// Simulating a Ticket Generation prompt
	prompt := `
	Goal: Create tickets for the following spec:
	### ID:[PRIMES] Prime Number Script
	CRITICAL INSTRUCTION: You MUST create exactly ONE ticket. Type: Task.
	`
	response, err := agent.Send(context.Background(), prompt)

	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Should return JSON
	if !strings.HasPrefix(strings.TrimSpace(response), "[") {
		t.Errorf("Expected JSON array for planning prompt, got: %s", response)
	}
	if !strings.Contains(response, "Create Prime Number Script") {
		t.Errorf("Expected ticket summary in JSON, got: %s", response)
	}
}

func TestMockAgent_Primes_Coding(t *testing.T) {
	agent := NewMockAgent()

	// Simulating an Implementation prompt
	prompt := `
	You are an agent working on ticket [PRIMES].
	Implement the script 'primes.py'.
	`
	response, err := agent.Send(context.Background(), prompt)

	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Should return Bash
	if !strings.Contains(response, "cat << 'EOF' > primes.py") {
		t.Errorf("Response missing primes.py creation, got: %s", response)
	}

	if !strings.Contains(response, "python3 primes.py") {
		t.Errorf("Response missing execution command, got: %s", response)
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
