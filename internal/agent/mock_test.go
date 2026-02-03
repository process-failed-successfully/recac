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
		expectContains []string
	}{
		{
			name:           "QA Agent",
			prompt:         "## YOUR ROLE - QA AGENT\nPlease verify...",
			expectContains: []string{"QA checks passed", "agent-bridge signal QA_PASSED true"},
		},
		{
			name:           "Project Manager",
			prompt:         "## YOUR ROLE - PROJECT MANAGER\nPlease sign off...",
			expectContains: []string{"Project signed off", "agent-bridge signal PROJECT_SIGNED_OFF true"},
		},
		{
			name:           "TPM",
			prompt:         "You are the Technical Program Manager",
			expectContains: []string{"PRIMES", "Implement Prime Number Generator"},
		},
		{
			name:           "Initializer",
			prompt:         "You are the Initializer agent",
			expectContains: []string{"feature_list.json"},
		},
		{
			name:           "Implementation",
			prompt:         "Please implement primes.py",
			expectContains: []string{"def is_prime(n):", "req-primes-json-contains-correct-p"},
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
					t.Errorf("Expected response to contain '%s', got:\n%s", exp, resp)
				}
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
