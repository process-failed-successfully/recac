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
	}{
		{
			name:           "TPM Planning",
			prompt:         "You are an expert Technical Program Manager. Plan the task.",
			expectedOutput: "ID:[PRIMES]",
		},
		{
			name:           "Initializer",
			prompt:         "## YOUR ROLE - INITIALIZER AGENT\n\nCreate features.",
			expectedOutput: "agent-bridge import feature_list.json",
		},
		{
			name:           "Coding Agent",
			prompt:         "## YOUR ROLE - CODING AGENT\n\nImplement primes.py",
			expectedOutput: "def is_prime(n):",
		},
		{
			name:           "QA Agent",
			prompt:         "## YOUR ROLE - QA AGENT\n\nVerify.",
			expectedOutput: "QA_PASSED",
		},
		{
			name:           "Project Manager",
			prompt:         "## YOUR ROLE - PROJECT MANAGER\n\nReview.",
			expectedOutput: "Approved",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := agent.Send(ctx, tt.prompt)
			if err != nil {
				t.Fatalf("Send failed: %v", err)
			}
			if !strings.Contains(resp, tt.expectedOutput) {
				t.Errorf("Expected response to contain %q, got:\n%s", tt.expectedOutput, resp)
			}
		})
	}
}
