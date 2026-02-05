package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent_Basic(t *testing.T) {
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

func TestMockAgent_SmartLogic(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	tests := []struct {
		name           string
		prompt         string
		expectContains []string
	}{
		{
			name:   "Initializer",
			prompt: "Your role is Initializer Agent. Please setup the repo.",
			expectContains: []string{
				"git init",
				"cat <<EOF | agent-bridge import",
				"```bash",
				`"project_name": "Mock Project"`,
			},
		},
		{
			name:   "TPM - Primes",
			prompt: "You are the Technical Program Manager. The task is [PRIMES].",
			expectContains: []string{
				`"id": "PRIMES"`,
				`"assigned_to": "Developer"`,
			},
		},
		{
			name:   "TPM - Default",
			prompt: "You are the Technical Program Manager. What is the plan?",
			expectContains: []string{
				`"id": "TASK-1"`,
			},
		},
		{
			name:   "Developer - Primes",
			prompt: "You are the Developer. Implement the prime number calculator. [PRIMES]",
			expectContains: []string{
				"cat <<EOF > primes.py",
				"def is_prime(n):",
				"python3 primes.py",
				"agent-bridge feature set PRIMES implemented",
			},
		},
		{
			name:   "QA Agent",
			prompt: "Your role is QA Agent. Verify the changes.",
			expectContains: []string{
				"All tests passed",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := agent.Send(ctx, tt.prompt)
			if err != nil {
				t.Fatalf("Send failed: %v", err)
			}
			for _, exp := range tt.expectContains {
				if !strings.Contains(resp, exp) {
					t.Errorf("Expected response to contain %q, got:\n%s", exp, resp)
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
