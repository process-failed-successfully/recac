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

func TestMockAgent_Primes_Initializer(t *testing.T) {
	agent := NewMockAgent()

	// Simulate Initializer prompt (contains [PRIMES] from spec and Role header)
	prompt := `## YOUR ROLE - INITIALIZER AGENT
### Application Specification:
### ID:[PRIMES] Prime Number Script
...`

	response, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Should return JSON (Planning), NOT Bash (Implementation)
	if !strings.Contains(response, "```json") {
		t.Errorf("Expected JSON response for Initializer, got: %s", response)
	}
	if strings.Contains(response, "cat << 'EOF' > primes.py") {
		t.Errorf("Received implementation script in Initializer phase!")
	}
}

func TestMockAgent_Primes_Completion(t *testing.T) {
	agent := NewMockAgent()

	// Simulate prompt with history showing "No changes to commit"
	prompt := `## YOUR ROLE - CODING AGENT
[PRIMES]
...
Output:
No changes to commit
`

	response, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Should detect completion
	if !strings.Contains(response, "Task appears complete") {
		t.Errorf("Expected completion response, got: %s", response)
	}
}
