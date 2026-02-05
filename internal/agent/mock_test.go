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

func TestMockAgent_SmartPrimes(t *testing.T) {
	agent := NewMockAgent()

	prompts := []string{
		"Please implement the [PRIMES] task.",
		"Implement prime-python scenario.",
		"Create a Prime Number Script",
	}

	for _, p := range prompts {
		resp, err := agent.Send(context.Background(), p)
		if err != nil {
			t.Errorf("Send failed for prompt '%s': %v", p, err)
		}

		if !strings.Contains(resp, "primes.py") {
			t.Errorf("Response for '%s' should contain primes.py", p)
		}
		if !strings.Contains(resp, "cat << 'EOF' > primes.py") {
			t.Errorf("Response for '%s' should contain bash heredoc", p)
		}
		if !strings.Contains(resp, "git commit") {
			t.Errorf("Response for '%s' should contain git commit", p)
		}
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
