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
		expectContains string
	}{
		{
			name:           "Primes Implementation",
			prompt:         "Please calculate primes and output to primes.json",
			expectContains: "cat <<EOF > primes.py",
		},
		{
			name:           "Primes with Ticket ID",
			prompt:         "Ticket: [PRIMES] implement the script",
			expectContains: "cat <<EOF > primes.py",
		},
		{
			name:           "Primes Guard Clause",
			prompt:         "Please calculate primes. History: I will implement the prime number calculation script. ```bash cat <<EOF > primes.py ... ``` COMPLETED",
			expectContains: "agent-bridge signal COMPLETED true",
		},
		{
			name:           "Bootstrap (Generic Coding Agent)",
			prompt:         "## YOUR ROLE - CODING AGENT\n\n### STEP 1: GET YOUR BEARINGS (MANDATORY)",
			expectContains: "cat feature_list.json",
		},
		{
			name:           "QA Agent",
			prompt:         "## YOUR ROLE - QA AGENT",
			expectContains: "agent-bridge signal set QA_PASSED true",
		},
		{
			name:           "Project Manager",
			prompt:         "## ROLE: PROJECT MANAGER",
			expectContains: "agent-bridge signal set PROJECT_SIGNED_OFF true",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := agent.Send(ctx, tt.prompt)
			if err != nil {
				t.Fatalf("Send failed: %v", err)
			}
			if !strings.Contains(resp, tt.expectContains) {
				t.Errorf("Response does not contain expected string.\nExpected: %s\nGot: %s", tt.expectContains, resp)
			}
		})
	}
}
