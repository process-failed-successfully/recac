package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent_SmartHeuristics(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	tests := []struct {
		name     string
		prompt   string
		contains []string
	}{
		{
			name:     "Technical Program Manager",
			prompt:   "You are the Technical Program Manager... please break down the requirements...",
			contains: []string{"req-primes", "Implement Prime Number Generator"},
		},
		{
			name:     "Lead Software Architect",
			prompt:   "You are the Lead Software Architect... generate feature_list.json...",
			contains: []string{"cat <<EOF > feature_list.json", "task-primes", "pending"},
		},
		{
			name:     "Primes Task",
			prompt:   "Implement prime calculation logic in primes.py",
			contains: []string{"def is_prime(n):", "cat <<EOF > primes.py", "git commit"},
		},
		{
			name:     "Pending Status",
			prompt:   `Current status: { "features": [ { "status": "pending" } ] }`,
			contains: []string{"sed -i", "status", "done"},
		},
		{
			name:     "QA Agent",
			prompt:   "You are the QA AGENT",
			contains: []string{"agent-bridge signal QA_PASSED true"},
		},
		{
			name:     "Manager Agent",
			prompt:   "You are the Manager Agent",
			contains: []string{"agent-bridge signal PROJECT_SIGNED_OFF true"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := agent.Send(ctx, tt.prompt)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			for _, s := range tt.contains {
				if !strings.Contains(resp, s) {
					t.Errorf("response missing expected string %q. Got:\n%s", s, resp)
				}
			}
		})
	}
}
