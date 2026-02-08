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

func TestMockAgentHeuristics(t *testing.T) {
	ctx := context.Background()
	agent := NewMockAgent()

	tests := []struct {
		name     string
		prompt   string
		contains []string
	}{
		{
			name:     "TPM Tickets",
			prompt:   "You are a Technical Program Manager. Create tickets.",
			contains: []string{"ID:[req-must-correctly-identify-prime-]", "Implement Prime Number Checker"},
		},
		{
			name:     "Initializer Feature List",
			prompt:   "Create feature_list.json",
			contains: []string{"cat <<EOF > feature_list.json", "req-must-correctly-identify-prime-"},
		},
		{
			name:     "Coding Agent Primes",
			prompt:   "Write a python script to check for prime numbers.",
			contains: []string{"cat <<EOF > primes.py", "python3 primes.py", "agent-bridge feature set"},
		},
		{
			name:     "QA Agent",
			prompt:   "You are the QA AGENT. Verify the work.",
			contains: []string{"QA checks passed", "agent-bridge signal QA_PASSED true"},
		},
		{
			name:     "Manager Agent",
			prompt:   "You are the Project Manager. Approve the work.",
			contains: []string{"Approved", "agent-bridge signal PROJECT_SIGNED_OFF true"},
		},
		{
			name:     "No changes",
			prompt:   "git status says No changes to commit",
			contains: []string{"Done."},
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
					t.Errorf("Response missing '%s'. Got:\n%s", s, resp)
				}
			}
		})
	}
}
