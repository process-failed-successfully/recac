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
		name     string
		prompt   string
		contains string
	}{
		{
			name:     "TPM Role",
			prompt:   "You are an expert Technical Program Manager (TPM)",
			contains: "ID:[PRIMES] Implement Primes",
		},
		{
			name:     "Initializer Role",
			prompt:   "## YOUR ROLE - INITIALIZER AGENT",
			contains: "agent-bridge import",
		},
		{
			name:     "Coding Role - Primes Tag",
			prompt:   "[PRIMES] Implement this",
			contains: "cat << 'EOF' > primes.py",
		},
		{
			name:     "Coding Role - Role Header",
			prompt:   "## YOUR ROLE - CODING AGENT",
			contains: "python3 primes.py",
		},
		{
			name:     "Coding Role - File Name",
			prompt:   "Create primes.py",
			contains: "agent-bridge signal COMPLETED",
		},
		{
			name:     "QA Role",
			prompt:   "## YOUR ROLE - QA AGENT",
			contains: "agent-bridge signal QA_PASSED",
		},
		{
			name:     "Manager Role",
			prompt:   "## YOUR ROLE - PROJECT MANAGER",
			contains: "agent-bridge signal PROJECT_SIGNED_OFF",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := agent.Send(ctx, tt.prompt)
			if err != nil {
				t.Fatalf("Send failed: %v", err)
			}
			if !strings.Contains(resp, tt.contains) {
				t.Errorf("Expected response to contain %q, got: %s", tt.contains, resp)
			}
		})
	}
}
