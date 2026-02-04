package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent(t *testing.T) {
	agent := NewMockAgent()

	tests := []struct {
		name     string
		prompt   string
		contains string
	}{
		{
			name:     "Ticket Generation",
			prompt:   "You are an expert Technical Program Manager. Generate tickets...",
			contains: "ID:[PRIMES]",
		},
		{
			name:     "Planner",
			prompt:   "## ROLE: Lead Software Architect. Create a plan...",
			contains: "project_name",
		},
		{
			name:     "Initializer",
			prompt:   "Initializer: Create feature_list.json...",
			contains: "agent-bridge import",
		},
		{
			name:     "QA Agent",
			prompt:   "QA AGENT: Review the code...",
			contains: "signal QA_PASSED true",
		},
		{
			name:     "Project Manager Signoff",
			prompt:   "PROJECT MANAGER: Review for sign-off...",
			contains: "signal PROJECT_SIGNED_OFF true",
		},
		{
			name:     "Implementation",
			prompt:   "Coding Agent: Implement primes.py...",
			contains: "primes.py",
		},
		{
			name:     "Fallback",
			prompt:   "Hello world",
			contains: "Mock Agent processing",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := agent.Send(context.Background(), tt.prompt)
			if err != nil {
				t.Fatalf("Send failed: %v", err)
			}
			if !strings.Contains(resp, tt.contains) {
				t.Errorf("Response missing expected content %q. Got: %s", tt.contains, resp)
			}
		})
	}
}

func TestTruncateString(t *testing.T) {
	if got := truncateString("hello", 3); got != "hel" {
		t.Errorf("Expected 'hel', got %q", got)
	}
	if got := truncateString("hello", 10); got != "hello" {
		t.Errorf("Expected 'hello', got %q", got)
	}
}
