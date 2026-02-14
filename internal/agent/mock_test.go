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

func TestTruncateString(t *testing.T) {
	s := "hello world"
	if truncateString(s, 5) != "hello" {
		t.Errorf("Expected 'hello', got '%s'", truncateString(s, 5))
	}
	if truncateString(s, 20) != "hello world" {
		t.Errorf("Expected 'hello world', got '%s'", truncateString(s, 20))
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
		{"TPM", "You are an expert Technical Program Manager (TPM)", "ID:[PRIMES]"},
		{"Coding", "ID:[PRIMES] Please implement primes.py", "python3 primes.py"},
		{"QA", "## YOUR ROLE - QA AGENT\n\nInstructions...", "All tests passed"},
		{"Manager", "## YOUR ROLE - PROJECT MANAGER\n\nInstructions...", "Approve"},
		{"Coding-With-Mentions", "## YOUR ROLE - CODING AGENT\n\n3. **Manager Review**: agent-bridge manager (Triggers Manager Review).\n2. **Quality Assurance**: agent-bridge qa (Triggers QA Agent).\n\nTask: ID:[PRIMES]", "python3 primes.py"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := agent.Send(ctx, tt.prompt)
			if err != nil {
				t.Fatalf("Send failed: %v", err)
			}
			if !strings.Contains(resp, tt.contains) {
				t.Errorf("Expected response to contain '%s', got: %s", tt.contains, resp)
			}
		})
	}
}
