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
			name:   "TPM [PRIMES]",
			prompt: "## YOUR ROLE - TECHNICAL PROGRAM MANAGER ... [PRIMES] ...",
			expectContains: []string{
				`"id": "req-primes"`,
				`"title": "Implement Prime Number Function"`,
			},
		},
		{
			name:   "Initializer",
			prompt: "## YOUR ROLE - INITIALIZER ...",
			expectContains: []string{
				"cat <<EOF > feature_list.json",
				"agent-bridge import < feature_list.json",
			},
		},
		{
			name:   "Coding Agent - req-primes",
			prompt: "## YOUR ROLE - CODING AGENT ... req-primes ...",
			expectContains: []string{
				"cat <<EOF > primes.py",
				"def is_prime(n):",
				"agent-bridge feature set req-primes --status done",
			},
		},
		{
			name:   "Coding Agent - Loop Breaker",
			prompt: "## YOUR ROLE - CODING AGENT ... nothing to commit ...",
			expectContains: []string{
				"agent-bridge feature set req-primes --status done",
			},
		},
		{
			name:   "Project Manager",
			prompt: "## YOUR ROLE - PROJECT MANAGER ...",
			expectContains: []string{
				"agent-bridge signal --privileged PROJECT_SIGNED_OFF true",
			},
		},
		{
			name:   "QA",
			prompt: "## YOUR ROLE - QA ...",
			expectContains: []string{
				"agent-bridge signal create QA_PASSED true",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := agent.Send(ctx, tt.prompt)
			if err != nil {
				t.Fatalf("Send failed: %v", err)
			}
			for _, expected := range tt.expectContains {
				if !strings.Contains(resp, expected) {
					t.Errorf("Response mismatch for %s.\nExpected to contain: %s\nGot:\n%s", tt.name, expected, resp)
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
