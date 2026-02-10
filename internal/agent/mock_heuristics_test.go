package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent_Heuristics(t *testing.T) {
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
			expectContains: "app_spec.txt",
			expectJSON:     false,
		},
		{
			name:           "Coding Agent",
			prompt:         "## YOUR ROLE - CODING AGENT [PRIMES]",
			expectContains: "def is_prime(n):",
			expectJSON:     false,
		},
		{
			name:           "Coding Agent Git Push",
			prompt:         "## YOUR ROLE - CODING AGENT [PRIMES]",
			expectContains: "git push",
			expectJSON:     false,
		},
		{
			name:           "QA Agent",
			prompt:         "## YOUR ROLE - QA AGENT",
			expectContains: "agent-bridge signal --privileged QA_PASSED true",
			expectJSON:     false,
		},
		{
			name:           "Project Manager",
			prompt:         "## YOUR ROLE - PROJECT MANAGER",
			expectContains: "APPROVED",
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
			agent := NewMockAgent()
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

func TestMockAgent_Stateful(t *testing.T) {
	agent := NewMockAgent()
	prompt := "## YOUR ROLE - CODING AGENT [PRIMES]"

	// 1st Call: Script
	resp1, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("First call failed: %v", err)
	}
	if !strings.Contains(resp1, "def is_prime(n):") {
		t.Errorf("First response expected script, got: %s", resp1)
	}

	// 2nd Call: Done
	resp2, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Second call failed: %v", err)
	}
	if !strings.Contains(resp2, "Task completed") {
		t.Errorf("Second response expected 'Task completed', got: %s", resp2)
	}
	if !strings.Contains(resp2, "QA_PASSED") {
		t.Errorf("Second response expected 'QA_PASSED', got: %s", resp2)
	}
}
