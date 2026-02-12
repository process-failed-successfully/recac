package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMockAgent_Heuristic(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	tests := []struct {
		name           string
		prompt         string
		expectContains []string
	}{
		{
			name:           "TPM Role",
			prompt:         "You are the Technical Program Manager. Generate a spec.",
			expectContains: []string{"[", "{", "\"type\": \"Epic\"", "\"id\": \"PRIMES\""},
		},
		{
			name:           "Initializer Role",
			prompt:         "## YOUR ROLE - INITIALIZER AGENT",
			expectContains: []string{"feature_list.json", "primes-script", "agent-bridge import"},
		},
		{
			name:           "Coding Agent - Prime Python",
			prompt:         "Write a python script to calculate primes.",
			expectContains: []string{"def is_prime(n):", "primes.json", "agent-bridge feature set"},
		},
		{
			name:           "QA Agent",
			prompt:         "Please REVIEW the changes.",
			expectContains: []string{"LGTM", "QA_PASSED"},
		},
		{
			name:           "Fallback",
			prompt:         "Hello there.",
			expectContains: []string{"Mock agent response:", "I received your prompt"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := agent.Send(ctx, tt.prompt)
			assert.NoError(t, err)
			for _, exp := range tt.expectContains {
				assert.Contains(t, resp, exp)
			}

			// For TPM, verify it's valid JSON start
			if strings.Contains(tt.prompt, "TPM") {
				assert.True(t, strings.HasPrefix(strings.TrimSpace(resp), "["), "TPM response should start with [")
			}
		})
	}
}
