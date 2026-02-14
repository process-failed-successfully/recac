package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent(t *testing.T) {
	agent := NewMockAgent()

	prompt := "This is a test prompt that is long enough to be truncated"
	response, err := agent.Send(context.Background(), prompt)

	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(response, "Mock agent response") {
		t.Errorf("Response missing prefix, got: %s", response)
	}

	if !strings.Contains(response, "I received your prompt") {
		t.Errorf("Response missing body, got: %s", response)
	}
}

func TestMockAgent_Heuristics(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	tests := []struct {
		name     string
		prompt   string
		contains []string
	}{
		{
			name:     "Initializer",
			prompt:   "You are the Initializer Agent. Please set up the project.",
			contains: []string{"agent-bridge import", "#!/bin/bash"},
		},
		{
			name:     "TPM",
			prompt:   "You are the TPM. Generate Jira tickets from this spec.",
			contains: []string{"ID:[PRIMES]", "Task"},
		},
		{
			name:     "Coding",
			prompt:   "Write a python script to calculate PRIMES.",
			contains: []string{"def is_prime(n):", "import json"},
		},
		{
			name:     "QA",
			prompt:   "You are the QA Agent. Verify the code.",
			contains: []string{"QA_PASSED=true"},
		},
		{
			name:     "Manager",
			prompt:   "You are the Manager Agent. Review and sign off.",
			contains: []string{"agent-bridge feature set", "passed"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := agent.Send(ctx, tt.prompt)
			if err != nil {
				t.Fatalf("Send failed: %v", err)
			}
			for _, s := range tt.contains {
				if !strings.Contains(resp, s) {
					t.Errorf("Response missing '%s'. Got:\n%s", s, resp)
				}
			}
		})
	}
}

func TestTruncateString(t *testing.T) {
	s := "hello world"
	if truncateString(s, 5) != "hello" {
		t.Errorf("Expected 'hello', got '%s'", truncateString(s, 5))
	}
	if truncateString(s, 20) != "hello world" {
		t.Errorf("Expected 'hello world', got '%s'", truncateString(s, 20))
	}
}
