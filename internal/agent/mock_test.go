package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMockAgent_Send(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	tests := []struct {
		name           string
		prompt         string
		expectedOutput string
		excludedOutput string
	}{
		{
			name:           "Nothing to commit",
			prompt:         "git status: nothing to commit, working tree clean",
			expectedOutput: "agent-bridge signal QA_PASSED true",
		},
		{
			name:           "Initializer Agent",
			prompt:         "You are the Initializer Agent",
			expectedOutput: "agent-bridge import",
		},
		{
			name:           "Technical Program Manager",
			prompt:         "You are the Technical Program Manager. Generate ticket.",
			expectedOutput: `"title": "Implement Prime Number Generator"`,
		},
		{
			name:           "Primes Task",
			prompt:         "Implement a function to calculate primes.",
			expectedOutput: "cat <<EOF > primes.py",
		},
		{
			name:           "QA Agent",
			prompt:         "You are the QA Agent. Review the code.",
			expectedOutput: "agent-bridge signal QA_PASSED true",
		},
		{
			name:           "Manager Review Conflict",
			prompt:         "You are the Manager Review agent. Review the primes implementation.",
			expectedOutput: "agent-bridge signal PROJECT_SIGNED_OFF true",
			excludedOutput: "cat <<EOF > primes.py",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := agent.Send(ctx, tt.prompt)
			require.NoError(t, err)
			assert.Contains(t, resp, tt.expectedOutput)
			if tt.excludedOutput != "" {
				assert.NotContains(t, resp, tt.excludedOutput)
			}
		})
	}
}

func TestMockAgent_Heuristics_Priority(t *testing.T) {
	// This test explicitly verifies that Manager Review takes precedence over Primes coding
	agent := NewMockAgent()
	ctx := context.Background()

	prompt := "Manager Review: The feature is to calculate primes."

	// Currently this fails because "primes" heuristic matches first
	resp, err := agent.Send(ctx, prompt)
	require.NoError(t, err)

	// We expect the Manager heuristic to trigger (sign off), NOT the coding heuristic
	if strings.Contains(resp, "cat <<EOF > primes.py") {
		t.Log("FAIL: Agent triggered coding heuristic instead of manager review")
	}

	assert.Contains(t, resp, "agent-bridge signal PROJECT_SIGNED_OFF true", "Should trigger Manager heuristic")
	assert.NotContains(t, resp, "cat <<EOF > primes.py", "Should NOT trigger coding heuristic")
}
