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
	ctx := context.Background()

	tests := []struct {
		name     string
		prompt   string
		expected string
	}{
		{
			name:     "TPM Phase",
			prompt:   "You are a Technical Program Manager. Respond with JSON format.",
			expected: "[",
		},
		{
			name:     "Commit Message Phase",
			prompt:   "Write a commit message for these changes.",
			expected: "feat: Implement primes.py",
		},
		{
			name:     "Coding Phase - Generate",
			prompt:   "Create a script to generate prime numbers.",
			expected: "```python",
		},
		{
			name:     "Coding Phase - Already Generated",
			prompt:   "Create primes.py. Context: def generate_primes(n): ...",
			expected: "Task completed",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := agent.Send(ctx, tc.prompt)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if !strings.Contains(resp, tc.expected) {
				t.Errorf("Expected response to contain '%s', got: %s", tc.expected, resp)
			}
		})
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
