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
			name:     "TPM Role",
			prompt:   "You are a Technical Program Manager",
			contains: []string{`[{"title":`, "Primes", `"type": "Task"}`},
		},
		{
			name:     "Coding Task",
			prompt:   "Create a python script primes.py",
			contains: []string{"```bash", "echo", "primes.py", "git add", "git commit"},
		},
		{
			name:     "Completion",
			prompt:   "nothing to commit, working tree clean",
			contains: []string{"Task completed."},
		},
		{
			name:     "Fallback",
			prompt:   "Just a random chat",
			contains: []string{"I received your prompt", "Mock agent response"},
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
					t.Errorf("Response for %q missing %q. Got:\n%s", tt.prompt, s, resp)
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
