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
			name:     "Project Manager Role",
			prompt:   "You are acting as role - Project Manager",
			contains: []string{"tickets", "Implement Prime Number Script", "PRIMES-1"},
		},
		{
			name:     "QA Agent Role",
			prompt:   "You are acting as role - QA Agent",
			contains: []string{"QA Approved", "PROJECT_SIGNED_OFF"},
		},
		{
			name:     "Coding Agent - ID:[PRIMES]",
			prompt:   "Task ID:[PRIMES]",
			contains: []string{"primes.py", "git commit", "PROJECT_SIGNED_OFF"},
		},
		{
			name:     "Coding Agent - primes.py",
			prompt:   "Please update primes.py",
			contains: []string{"primes.py", "git commit", "PROJECT_SIGNED_OFF"},
		},
		{
			name:     "Coding Agent - Prime Number Script",
			prompt:   "Task: Implement Prime Number Script",
			contains: []string{"primes.py", "git commit", "PROJECT_SIGNED_OFF"},
		},
		{
			name:     "Coding Agent - Injected Feature ID",
			prompt:   "Feature: req-script-runs-without-errors",
			contains: []string{"primes.py", "git commit", "PROJECT_SIGNED_OFF"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := agent.Send(ctx, tt.prompt)
			if err != nil {
				t.Fatalf("Send failed: %v", err)
			}
			for _, substr := range tt.contains {
				if !strings.Contains(resp, substr) {
					t.Errorf("Response missing expected substring %q. Got:\n%s", substr, resp)
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
