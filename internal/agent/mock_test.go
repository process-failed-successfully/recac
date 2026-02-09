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

func TestMockAgent_SmokeTestHeuristics(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	tests := []struct {
		name           string
		prompt         string
		expectContains string
	}{
		{
			name: "Initializer",
			prompt: "ROLE - INITIALIZER AGENT",
			expectContains: "agent-bridge import",
		},
		{
			name: "TPM Ticket Generation (Mixed Case)",
			prompt: "You are an expert Technical Program Manager... I want a PRIMES script outputting JSON",
			expectContains: "Implement a python script named 'primes.py'",
		},
		{
			name: "TPM Ticket Generation (Upper Case)",
			prompt: "ROLE - TECHNICAL PROGRAM MANAGER... I want a PRIMES script outputting JSON",
			expectContains: "Implement a python script named 'primes.py'",
		},
		{
			name: "Coding Agent - Implementation",
			prompt: "## YOUR ROLE - CODING AGENT\nTask: Implement primes.py",
			expectContains: "cat << 'EOF' > primes.py",
		},
		{
			name: "Coding Agent - With Review Instruction (Regression Test)",
			// This mimics the actual template which includes instructions about "Manager Review" and "SELF-REVIEW"
			prompt: "## YOUR ROLE - CODING AGENT\nTask: Implement primes.py\n... 4. **SELF-REVIEW**: review your code...",
			expectContains: "cat << 'EOF' > primes.py",
		},
		{
			name: "Manager Review",
			prompt: "ROLE - PROJECT MANAGER\nReview the work.",
			expectContains: "agent-bridge signal PROJECT_SIGNED_OFF true",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := agent.Send(ctx, tt.prompt)
			if err != nil {
				t.Fatalf("Send failed: %v", err)
			}
			if !strings.Contains(resp, tt.expectContains) {
				t.Errorf("Response did not contain expected string '%s'. Got:\n%s", tt.expectContains, resp)
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
