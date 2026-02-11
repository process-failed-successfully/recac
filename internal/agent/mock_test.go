package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent_Default(t *testing.T) {
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

func TestMockAgent_Roles(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	tests := []struct {
		name          string
		prompt        string
		expectContent string
	}{
		{
			name:          "TPM Role",
			prompt:        "You are an expert Technical Program Manager (TPM). ID:[TEST-1]",
			expectContent: "ID:[TEST-1] Implement Primes",
		},
		{
			name:          "Initializer Role",
			prompt:        "## YOUR ROLE - INITIALIZER AGENT",
			expectContent: "```bash",
		},
		{
			name:          "Coding Role",
			prompt:        "## YOUR ROLE - CODING AGENT",
			expectContent: "```bash",
		},
		{
			name:          "QA Role",
			prompt:        "## YOUR ROLE - QA AGENT",
			expectContent: "QA_PASSED",
		},
		{
			name:          "Manager Role",
			prompt:        "## YOUR ROLE - PROJECT MANAGER",
			expectContent: "PROJECT_SIGNED_OFF",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := agent.Send(ctx, tc.prompt)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if !strings.Contains(resp, tc.expectContent) {
				t.Errorf("Expected content %q not found in response: %s", tc.expectContent, resp)
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
