package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent_Heuristics(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	tests := []struct {
		name          string
		prompt        string
		expectContent []string
	}{
		{
			name:   "TPM - Project Manager",
			prompt: "You are a Technical Program Manager. Generate a ticket plan.",
			expectContent: []string{
				`"id": "PRIMES"`,
				`"title": "[PRIMES] Create Prime Number Script"`,
			},
		},
		{
			name:   "Initializer - Feature List",
			prompt: "You are an Initializer Agent. Create a feature list.",
			expectContent: []string{
				"```bash",
				"cat << 'EOF' > feature_list.json",
				`"id": "primes-script"`,
				"agent-bridge import",
			},
		},
		{
			name:   "Coding Agent - Primes",
			prompt: "Implement the [PRIMES] feature using python.",
			expectContent: []string{
				"```bash",
				"cat << 'EOF' > primes.py",
				"import json",
				"def get_primes(n):",
				"git add primes.py primes.json",
				"agent-bridge feature set primes-script --status completed",
			},
		},
		{
			name:   "QA Agent - Verify",
			prompt: "Review the code and verify correctness.",
			expectContent: []string{
				"LGTM",
			},
		},
		{
			name:   "Unknown - Fallback",
			prompt: "What is the capital of France?",
			expectContent: []string{
				"I received your prompt",
				"In mock mode",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := agent.Send(ctx, tt.prompt)
			if err != nil {
				t.Fatalf("Send() error = %v", err)
			}

			for _, expect := range tt.expectContent {
				if !strings.Contains(resp, expect) {
					t.Errorf("Send() response missing expected content %q. Got:\n%s", expect, resp)
				}
			}
		})
	}
}
