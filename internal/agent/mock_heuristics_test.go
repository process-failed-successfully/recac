package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent_Heuristics(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	tests := []struct {
		name     string
		prompt   string
		expected string
	}{
		{
			name:     "Planning Phase",
			prompt:   "You are an expert Technical Program Manager. Please generate a plan in json format.",
			expected: "[",
		},
		{
			name:     "Coding Phase",
			prompt:   "Write a python script to generate primes.json",
			expected: "cat <<EOF > primes.py",
		},
		{
			name:     "Commit Phase",
			prompt:   "Generate a commit message for the changes.",
			expected: "feat: Implement primes.py",
		},
		{
			name:     "Default",
			prompt:   "Hello world",
			expected: "Mock agent response:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := agent.Send(ctx, tt.prompt)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !strings.Contains(resp, tt.expected) {
				t.Errorf("expected response to contain %q, got %q", tt.expected, resp)
			}
		})
	}
}
