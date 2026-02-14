package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent(t *testing.T) {
	agent := NewMockAgent()

	prompt := "This is a random test prompt"
	response, err := agent.Send(context.Background(), prompt)

	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(response, "Mock Agent received prompt") {
		t.Errorf("Response missing prefix, got: %s", response)
	}
}

func TestMockAgent_Heuristics(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	tests := []struct {
		name     string
		prompt   string
		contains []string
	}{
		{
			name:     "Completion",
			prompt:   "nothing to commit here",
			contains: []string{"PROJECT_SIGNED_OFF"},
		},
		{
			name:     "Initializer",
			prompt:   "You are the Initializer Agent",
			contains: []string{"agent-bridge import", "prime-script"},
		},
		{
			name:     "Coding",
			prompt:   "Write a Prime Number Script",
			contains: []string{"primes.json", "def is_prime"},
		},
		{
			name:     "Manager",
			prompt:   "I am the Manager Agent",
			contains: []string{"agent-bridge feature set", "PROJECT_SIGNED_OFF"},
		},
		{
			name:     "TPM",
			prompt:   "recac generate-from-spec",
			contains: []string{"ID:[PRIMES]", "Implement Primes"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := agent.Send(ctx, tc.prompt)
			if err != nil {
				t.Fatalf("Send failed: %v", err)
			}
			for _, s := range tc.contains {
				if !strings.Contains(resp, s) {
					t.Errorf("Response missing '%s', got:\n%s", s, resp)
				}
			}
		})
	}
}
