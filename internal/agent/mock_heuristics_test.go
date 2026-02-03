package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent_Send_Heuristics(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	tests := []struct {
		name           string
		prompt         string
		expectedOutput []string
	}{
		{
			name:   "TPM Prompt with Repo URL",
			prompt: "You are the Technical Program Manager. Repo: https://github.com/test/repo",
			expectedOutput: []string{
				"ID:[PRIMES]",
				"https://github.com/test/repo",
			},
		},
		{
			name:   "Implementation Prompt (Primes)",
			prompt: "Create primes.py and implement the logic.",
			expectedOutput: []string{
				"cat << 'EOF' > primes.py",
				"python3 primes.py",
				"agent-bridge feature update req-create-primes-py done",
			},
		},
		{
			name:   "QA Prompt",
			prompt: "I am the QA Agent. Verify the project.",
			expectedOutput: []string{
				"agent-bridge signal QA_PASSED true",
			},
		},
		{
			name:   "Manager Prompt",
			prompt: "I am the Project Manager. Review the work.",
			expectedOutput: []string{
				"agent-bridge signal PROJECT_SIGNED_OFF true",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := agent.Send(ctx, tt.prompt)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			for _, expected := range tt.expectedOutput {
				if !strings.Contains(resp, expected) {
					t.Errorf("response missing expected content '%s'. Got:\n%s", expected, resp)
				}
			}
		})
	}
}
