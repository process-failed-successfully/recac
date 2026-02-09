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
		name           string
		prompt         string
		expectedOutput string
		contains       bool
	}{
		{
			name:           "TPM JSON",
			prompt:         "You are an expert Technical Program Manager (TPM)...",
			expectedOutput: "[", // Should start with JSON array
			contains:       true,
		},
		{
			name:           "Initializer Bash",
			prompt:         "ROLE - INITIALIZER AGENT",
			expectedOutput: "cat <<EOF > feature_list.json",
			contains:       true,
		},
		{
			name:           "Coding Agent",
			prompt:         "ROLE - CODING AGENT. Implement Primes",
			expectedOutput: "def is_prime(n):",
			contains:       true,
		},
		{
			name:           "QA Agent",
			prompt:         "ROLE - QA AGENT",
			expectedOutput: "QA_PASSED",
			contains:       false, // Exact match
		},
		{
			name:           "Project Manager",
			prompt:         "ROLE - PROJECT MANAGER",
			expectedOutput: "PROJECT_SIGNED_OFF",
			contains:       false, // Exact match
		},
		{
			name:           "Default",
			prompt:         "Hello there",
			expectedOutput: "Mock agent response",
			contains:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := agent.Send(ctx, tt.prompt)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.contains {
				if !strings.Contains(resp, tt.expectedOutput) {
					t.Errorf("expected response to contain %q, got %q", tt.expectedOutput, resp)
				}
			} else {
				if resp != tt.expectedOutput {
					t.Errorf("expected response to be %q, got %q", tt.expectedOutput, resp)
				}
			}
		})
	}
}
