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
		contains string
	}{
		{
			name:     "TPM Heuristic",
			prompt:   "You are an expert Technical Program Manager (TPM)...",
			contains: `[{"id":"TASK-1"`,
		},
		{
			name:     "Primes Heuristic",
			prompt:   "Please generate primes.json",
			contains: `{"primes": [2`,
		},
		{
			name:     "QA Heuristic",
			prompt:   "You are the QA Agent",
			contains: "QA_PASSED",
		},
		{
			name:     "Manager Review Heuristic",
			prompt:   "## your role - project manager",
			contains: "agent-bridge signal PROJECT_SIGNED_OFF true --privileged",
		},
		{
			name:     "Default Response",
			prompt:   "Hello world",
			contains: "Mock agent response",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := agent.Send(ctx, tt.prompt)
			if err != nil {
				t.Fatalf("Send failed: %v", err)
			}
			if !strings.Contains(resp, tt.contains) {
				t.Errorf("Expected response to contain %q, got %q", tt.contains, resp)
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
