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

func TestMockAgent_Heuristics_Completion(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	tests := []struct {
		name           string
		prompt         string
		expectedSignal string
	}{
		{
			name: "Standard Completion",
			prompt: `## YOUR ROLE - CODING AGENT
Task: Implement primes
History:
> Add primes script
> Output: [main 1234567] Add primes script
> git commit ... Success
`,
			expectedSignal: "PROJECT_SIGNED_OFF",
		},
		{
			name: "Idempotent Completion (Nothing to commit)",
			prompt: `## YOUR ROLE - CODING AGENT
Task: Implement primes
History:
> Add primes script
> Output: On branch agent/TEST
> nothing to commit, working tree clean
`,
			expectedSignal: "PROJECT_SIGNED_OFF",
		},
		{
			name: "Incomplete (First Run)",
			prompt: `## YOUR ROLE - CODING AGENT
Task: Implement primes
History: None
`,
			expectedSignal: "cat <<EOF > primes.py", // Expect code generation script
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := agent.Send(ctx, tt.prompt)
			if err != nil {
				t.Fatalf("Send failed: %v", err)
			}
			if !strings.Contains(resp, tt.expectedSignal) {
				t.Errorf("Expected response to contain '%s', got:\n%s", tt.expectedSignal, resp)
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
