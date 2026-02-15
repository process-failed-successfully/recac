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
		name     string
		prompt   string
		contains []string
	}{
		{
			name:     "Initializer",
			prompt:   "You are an Initializer Agent. Create a plan.",
			contains: []string{`"plan":`, `"task-1"`, `"Implement the requested feature"`},
		},
		{
			name:     "TPM",
			prompt:   "You are a TPM. Provide feature breakdown.",
			// Expecting JSON array of tickets now (compact JSON)
			contains: []string{`"title":"ID:[PRIMES] Generate Primes"`, `"type":"Epic"`, `"children":[`},
		},
		{
			name:     "QA",
			prompt:   "You are a QA Agent. Verify this code.",
			contains: []string{"agent-bridge signal --success", "Verification passed"},
		},
		{
			name:     "Manager",
			prompt:   "You are a Manager Agent. Review this PR.",
			contains: []string{"agent-bridge signal --approve", "Approved"},
		},
		{
			name:     "Coding_Primes",
			prompt:   "Write a python script to generate prime numbers and save to primes.json.",
			contains: []string{"def is_prime(n):", "primes = [i for i in range(10001)", `with open("primes.json", "w")`},
		},
		{
			name:     "NothingToCommit",
			prompt:   "nothing to commit, working tree clean",
			contains: []string{`"action": "commit"`, `"No changes to commit"`},
		},
		{
			name:     "Default",
			prompt:   "Hello world",
			contains: []string{"Mock agent response", "I received your prompt"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := agent.Send(ctx, tt.prompt)
			if err != nil {
				t.Fatalf("Send failed: %v", err)
			}
			for _, substr := range tt.contains {
				if !strings.Contains(resp, substr) {
					t.Errorf("Response missing substring %q. Got:\n%s", substr, resp)
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
