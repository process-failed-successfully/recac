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
		name           string
		prompt         string
		expectedOutput string // Substring match
		notExpected    string // Should NOT contain this substring
	}{
		{
			name:           "TPM Agent - Generates Tickets",
			prompt:         "You are a Technical Program Manager. Please create tickets for prime number generator.",
			expectedOutput: "ID:[PRIMES]",
			notExpected:    "Mock agent response",
		},
		{
			name:           "TPM Agent - Correct Repo URL",
			prompt:         "You are a Technical Program Manager. Please create tickets for prime number generator.",
			expectedOutput: "https://github.com/process-failed-successfully/recac-jira-e2e",
		},
		{
			name:           "Initializer Agent - Extract Features",
			prompt:         "You are the Initializer Agent. Please extract features for prime number generator.",
			expectedOutput: `["PRIMES-1"]`,
			notExpected:    "def is_prime(n):", // Should NOT be code
		},
		{
			name:           "Architect Agent - Extract Features (New Heuristic)",
			prompt:         "You are the Architect. Please plan the solution for prime number generator.",
			expectedOutput: `["PRIMES-1"]`,
			notExpected:    "def is_prime(n):", // Should NOT be code
		},
		{
			name:           "Coding Agent - Primes",
			prompt:         "Please implement the prime number generator in python. Output to primes.json.",
			expectedOutput: "def is_prime(n):",
			notExpected:    "Mock agent response",
		},
		{
			name:           "Commit Agent - Message",
			prompt:         "Please generate a commit message for the changes.",
			expectedOutput: "feat: Implement primes.py",
			notExpected:    "Mock agent response",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := agent.Send(ctx, tt.prompt)
			if err != nil {
				t.Fatalf("Send failed: %v", err)
			}

			if !strings.Contains(resp, tt.expectedOutput) {
				t.Errorf("Expected output to contain %q, got:\n%s", tt.expectedOutput, resp)
			}

			if tt.notExpected != "" && strings.Contains(resp, tt.notExpected) {
				t.Errorf("Expected output NOT to contain %q, but it did:\n%s", tt.notExpected, resp)
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
