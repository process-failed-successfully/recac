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
			name:   "Primes Script",
			prompt: "Implement a python script for ID:[PRIMES]",
			contains: []string{
				"I will implement the prime number checking script",
				"```bash",
				"def is_prime(n):",
				"if __name__ == \"__main__\":",
				"> primes.py",
			},
		},
		{
			name:   "Git Lead",
			prompt: "You are the Git Lead. Please create a branch.",
			contains: []string{
				"I will ensure the feature branch is ready",
				"```bash",
				"git rev-parse",
			},
		},
		{
			name:   "Planner",
			prompt: "Generate feature_list.json for the planner",
			contains: []string{
				"project_name",
				"features",
				"feature-1",
			},
		},
		{
			name:   "TPM",
			prompt: "As Technical Program Manager, output purely JSON for the project.",
			contains: []string{
				"ID:[PRIMES]",
				"Prime Number Script",
				"Implement is_prime function",
			},
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
					t.Errorf("Response missing %q, got: %s", substr, resp)
				}
			}
		})
	}
}
