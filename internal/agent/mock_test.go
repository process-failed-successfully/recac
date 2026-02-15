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
	// Now expects "..." appended if truncated
	expectedTruncated := "hello..."
	if got := truncateString(s, 5); got != expectedTruncated {
		t.Errorf("Expected '%s', got '%s'", expectedTruncated, got)
	}

	// If not truncated, should be exact match
	if got := truncateString(s, 20); got != s {
		t.Errorf("Expected '%s', got '%s'", s, got)
	}
}

func TestMockAgent_Heuristics(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	tests := []struct {
		name           string
		prompt         string
		expectContains []string
	}{
		{
			name:   "TPM Prompt",
			prompt: "You are an expert Technical Program Manager. Please generate a plan in JSON format.",
			expectContains: []string{
				`"id": "TASK-1"`,
				`"type": "Task"`,
				`"repo_url": "https://github.com/process-failed-successfully/recac-jira-e2e"`,
			},
		},
		{
			name:   "Coding Prompt - Primes",
			prompt: "Generate a python script to calculate primes.json using the sieve method.",
			expectContains: []string{
				`"path": "primes.py"`,
				`def is_prime(n):`,
			},
		},
		{
			name:           "Commit Message Prompt",
			prompt:         "Generate a git commit message for these changes.",
			expectContains: []string{"feat: Implement primes.py"},
		},
		{
			name:           "Default Fallback",
			prompt:         "Just a random chat message.",
			expectContains: []string{"I received your prompt", "Mock agent response"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := agent.Send(ctx, tc.prompt)
			if err != nil {
				t.Fatalf("Send failed: %v", err)
			}
			for _, exp := range tc.expectContains {
				if !strings.Contains(resp, exp) {
					t.Errorf("Expected response to contain %q, but got:\n%s", exp, resp)
				}
			}
		})
	}
}
