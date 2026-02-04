package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent_E2EBehaviors(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	tests := []struct {
		name           string
		prompt         string
		wantSubstrings []string
	}{
		{
			name:   "Initializer",
			prompt: "Please initialize the project with feature_list.json",
			wantSubstrings: []string{
				"```bash",
				"cat <<EOF > /tmp/features.json",
				"agent-bridge feature import",
			},
		},
		{
			name:   "TPM",
			prompt: "You are the Technical Program Manager. Please provide a JSON plan.",
			wantSubstrings: []string{
				"\"tickets\": [",
				"\"id\": \"PRIMES\"",
			},
		},
		{
			name:   "Implementation",
			prompt: "Implement Prime Number Generator (primes.py)",
			wantSubstrings: []string{
				"```bash",
				"cat <<EOF > primes.py",
				"def is_prime(n):",
				"python3 primes.py",
				"git add primes.py",
			},
		},
		{
			name:   "QA",
			prompt: "I am the QA AGENT. Verify.",
			wantSubstrings: []string{
				"```bash",
				"agent-bridge signal set QA_PASSED true",
				"agent-bridge signal set PROJECT_SIGNED_OFF true",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := agent.Send(ctx, tt.prompt)
			if err != nil {
				t.Fatalf("Send failed: %v", err)
			}
			for _, want := range tt.wantSubstrings {
				if !strings.Contains(got, want) {
					t.Errorf("Response missing %q. Got:\n%s", want, got)
				}
			}
		})
	}
}
