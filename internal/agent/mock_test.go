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
		name           string
		prompt         string
		wantContain    []string
		wantNotContain []string
	}{
		{
			name:        "TPM Scenario",
			prompt:      "You are the Technical Program Manager. Create tickets.",
			wantContain: []string{"ID:[PRIMES]", "Implement Prime Number Generator", "Task", "In Progress"},
		},
		{
			name:        "Primes Implementation",
			prompt:      "Please work on ID:[PRIMES]. Check primes.py",
			wantContain: []string{"cat <<EOF > primes.py", "def is_prime(n):", "git add primes.py", "git commit -m"},
		},
		{
			name:        "Primes Completion",
			prompt:      "ID:[PRIMES] primes.py: nothing to commit, working tree clean",
			wantContain: []string{"agent-bridge feature set", "Done", "exit 0"},
		},
		{
			name:        "Default Fallback",
			prompt:      "Just a random chat message",
			wantContain: []string{"I received your prompt", "Mock agent response"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := agent.Send(ctx, tt.prompt)
			if err != nil {
				t.Fatalf("Send() error = %v", err)
			}
			for _, want := range tt.wantContain {
				if !strings.Contains(got, want) {
					t.Errorf("Send() response missing %q. Got:\n%s", want, got)
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
