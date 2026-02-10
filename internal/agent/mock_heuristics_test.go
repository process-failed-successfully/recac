package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent_Heuristics(t *testing.T) {
	agent := NewMockAgent()

	tests := []struct {
		name           string
		prompt         string
		expectContains string
		expectJSON     bool
	}{
		{
			name:           "TPM Agent",
			prompt:         "You are an expert Technical Program Manager... please generate a JSON list of Jira tickets",
			expectContains: `"title": "ID:[PRIMES] Implement Prime Number Script"`,
			expectJSON:     true,
		},
		{
			name:           "Initializer Agent",
			prompt:         "## YOUR ROLE - INITIALIZER AGENT",
			expectContains: "echo 'Initializing environment...'",
			expectJSON:     false,
		},
		{
			name:           "Coding Agent",
			prompt:         "## YOUR ROLE - CODING AGENT [PRIMES]",
			expectContains: "def is_prime(n):",
			expectJSON:     false,
		},
		{
			name:           "QA Agent",
			prompt:         "## YOUR ROLE - QA AGENT",
			expectContains: "agent-bridge signal --privileged QA_PASSED true",
			expectJSON:     false,
		},
		{
			name:           "Default Fallback",
			prompt:         "Hello there",
			expectContains: "I received your prompt",
			expectJSON:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := agent.Send(context.Background(), tt.prompt)
			if err != nil {
				t.Fatalf("Send failed: %v", err)
			}

			if !strings.Contains(resp, tt.expectContains) {
				t.Errorf("Response did not contain expected string.\nExpected: %s\nGot: %s", tt.expectContains, resp)
			}

			if tt.expectJSON {
				if strings.HasPrefix(resp, "Mock agent response") {
					t.Errorf("Expected JSON response but got default conversational response")
				}
			}
		})
	}
}
