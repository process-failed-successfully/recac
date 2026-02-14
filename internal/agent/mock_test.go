package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent_Send(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	tests := []struct {
		name           string
		prompt         string
		expectContains []string
	}{
		{
			name:           "Default Response",
			prompt:         "Hello world",
			expectContains: []string{"I received your prompt"},
		},
		{
			name:           "TPM Role",
			prompt:         "You are the Technical Program Manager. Generate tickets.",
			expectContains: []string{"ID:[PRIMES]", "Task", "primes.py"},
		},
		{
			name:           "Coding Agent",
			prompt:         "Implement the prime number script in python. ID:[PRIMES]",
			expectContains: []string{"cat << 'EOF' > primes.py", "python3 primes.py", "git commit", "PROJECT_SIGNED_OFF"},
		},
		{
			name:           "QA Agent",
			prompt:         "Your role - QA Agent. Verify functionality.",
			expectContains: []string{"agent-bridge signal QA_PASSED true"},
		},
		{
			name:           "Manager",
			prompt:         "Your role - Project Manager. Review the work.",
			expectContains: []string{"agent-bridge signal PROJECT_SIGNED_OFF true"},
		},
		{
			name:           "Initializer",
			prompt:         "You are the Initializer Agent. Initialize features.",
			expectContains: []string{"agent-bridge import feature_list.json"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := agent.Send(ctx, tt.prompt)
			if err != nil {
				t.Fatalf("Send() error = %v", err)
			}
			for _, exp := range tt.expectContains {
				if !strings.Contains(resp, exp) {
					t.Errorf("Send() response missing expected string: %s. Got: %s", exp, resp)
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
