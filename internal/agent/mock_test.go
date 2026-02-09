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
		expectedOutput string // Substring expected
	}{
		{
			name:           "Prime Python Scenario",
			prompt:         "Please implement the primes.py script as requested in [PRIMES] ticket.",
			expectedOutput: "cat << 'EOF' > primes.py",
		},
		{
			name:           "QA Verification",
			prompt:         "Please run QA and verify the implementation.",
			expectedOutput: "agent-bridge signal --privileged QA_PASSED true",
		},
		{
			name:           "Nothing to Commit (Loop Breaker)",
			prompt:         "The working tree is clean, nothing to commit.",
			expectedOutput: "agent-bridge signal --privileged QA_PASSED true",
		},
		{
			name:           "Project Sign Off",
			prompt:         "The project is ready. Project Signed Off.",
			expectedOutput: "agent-bridge signal --privileged PROJECT_SIGNED_OFF true",
		},
		{
			name:           "Feature List (Initializer)",
			prompt:         "Please generate the feature list in JSON format.",
			expectedOutput: "cat << 'EOF' > feature_list.json",
		},
		{
			name:           "Default",
			prompt:         "Hello world",
			expectedOutput: "I received your prompt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := agent.Send(ctx, tt.prompt)
			if err != nil {
				t.Fatalf("Send failed: %v", err)
			}
			if !strings.Contains(resp, tt.expectedOutput) {
				t.Errorf("expected response to contain %q, got: %q", tt.expectedOutput, resp)
			}
		})
	}
}
