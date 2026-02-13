package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent(t *testing.T) {
	agent := NewMockAgent()
	agent.SetResponse("custom response")

	ctx := context.Background()
	resp, err := agent.Send(ctx, "test prompt")
	if err != nil {
		t.Errorf("Send failed: %v", err)
	}
	if resp != "custom response" {
		t.Errorf("expected 'custom response', got '%s'", resp)
	}
}

func TestMockAgent_Heuristics(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	tests := []struct {
		name     string
		prompt   string
		expected string
	}{
		{
			name:     "Planning",
			prompt:   "I am a Technical Program Manager. Repo: https://github.com/test/repo",
			expected: "[PRIMES]",
		},
		{
			name:     "Execution",
			prompt:   "Please write a prime number python script.",
			expected: "cat << 'EOF' > primes.py",
		},
		{
			name:     "Completion_NothingToCommit",
			prompt:   "nothing to commit, working tree clean",
			expected: "Task completed",
		},
		{
			name:     "Completion_UpToDate",
			prompt:   "everything up-to-date",
			expected: "Task completed",
		},
		{
			name:     "Default",
			prompt:   "Just a regular prompt",
			expected: "Mock agent response",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := agent.Send(ctx, tt.prompt)
			if err != nil {
				t.Fatalf("Send failed: %v", err)
			}
			if !strings.Contains(resp, tt.expected) {
				t.Errorf("expected response to contain '%s', got '%s'", tt.expected, resp)
			}
		})
	}
}

func TestTruncateString(t *testing.T) {
	s := "hello world"
	if truncateString(s, 5) != "hello" {
		t.Errorf("expected 'hello', got '%s'", truncateString(s, 5))
	}
	if truncateString(s, 20) != "hello world" {
		t.Errorf("expected 'hello world', got '%s'", truncateString(s, 20))
	}
}
