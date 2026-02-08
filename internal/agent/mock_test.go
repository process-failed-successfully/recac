package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent(t *testing.T) {
	agent := NewMockAgent("", "", "")

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

func TestMockAgent_SmokeTestHeuristics(t *testing.T) {
	agent := NewMockAgent("", "", "")
	ctx := context.Background()

	tests := []struct {
		name           string
		prompt         string
		expectedSubstr []string
	}{
		{
			name:   "Initializer",
			prompt: "Your role is INITIALIZER. Get your bearings.",
			expectedSubstr: []string{
				"feature_list.json",
				"repository_url",
				"req-primes-implementation",
				"agent-bridge import < feature_list.json",
				"cat <<EOF",
			},
		},
		{
			name:   "TPM",
			prompt: "You are the Technical Program Manager. Create a ticket plan.",
			expectedSubstr: []string{
				"PRIMES",
				"Implement Prime Number Script",
				"10,000",
			},
		},
		{
			name:   "Coding Agent - Primes",
			prompt: "You are the Coding Agent. Implement the Primes script req-primes-implementation.",
			expectedSubstr: []string{
				"primes(10000)",
				"def primes(n):",
			},
		},
		{
			name:   "Loop Breaker - Nothing to commit",
			prompt: "git status: nothing to commit, working tree clean",
			expectedSubstr: []string{
				"agent-bridge feature set req-primes-implementation --status done --passes true",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := agent.Send(ctx, tc.prompt)
			if err != nil {
				t.Fatalf("Send failed: %v", err)
			}
			for _, sub := range tc.expectedSubstr {
				if !strings.Contains(resp, sub) {
					t.Errorf("Expected response to contain %q, but got: %s", sub, resp)
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
