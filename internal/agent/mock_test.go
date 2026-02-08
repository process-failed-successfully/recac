package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent_Simple(t *testing.T) {
	agent := NewMockAgent()

	prompt := "This is a generic test prompt"
	response, err := agent.Send(context.Background(), prompt)

	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(response, "Mock agent response") {
		t.Errorf("Response missing prefix, got: %s", response)
	}
}

func TestMockAgent_Heuristics(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	tests := []struct {
		name           string
		prompt         string
		expectContains []string
	}{
		{
			name:   "Initializer",
			prompt: "ROLE - INITIALIZER: Setup the project features.",
			expectContains: []string{
				"agent-bridge import",
				"req-setup-repo",
				"req-implement-primes",
			},
		},
		{
			name:   "Project Manager - Features",
			prompt: "ROLE - PROJECT MANAGER: Generate a list of features.",
			expectContains: []string{
				"req-setup-repo",
				"ID:[PRIMES]",
				"req-implement-tests",
			},
		},
		{
			name:   "Project Manager - Review",
			prompt: "ROLE - PROJECT MANAGER: Review the project status.",
			expectContains: []string{
				"agent-bridge signal PROJECT_SIGNED_OFF true --privileged",
			},
		},
		{
			name:   "Coding Agent - Setup Repo",
			prompt: "ROLE - CODING AGENT: Feature ID: req-setup-repo",
			expectContains: []string{
				"git init",
				"git remote add origin",
			},
		},
		{
			name:   "Coding Agent - CI Workflow",
			prompt: "ROLE - CODING AGENT: Feature ID: req-ci-workflow",
			expectContains: []string{
				"mkdir -p .github/workflows",
				"cat <<EOF > .github/workflows/ci.yml",
				"agent-bridge mark-done req-ci-workflow",
			},
		},
		{
			name:   "Coding Agent - Implement Tests",
			prompt: "ROLE - CODING AGENT: Feature ID: req-implement-tests",
			expectContains: []string{
				"mkdir -p tests",
				"cat <<EOF > tests/test_primes.py",
				"agent-bridge mark-done req-implement-tests",
			},
		},
		{
			name:   "Coding Agent - Implement Primes",
			prompt: "ROLE - CODING AGENT: Feature ID: req-implement-primes",
			expectContains: []string{
				"cat <<EOF > primes.py",
				"def is_prime(n):",
				"agent-bridge mark-done req-implement-primes",
			},
		},
		{
			name:   "QA Agent",
			prompt: "ROLE - QA AGENT: Verify the project.",
			expectContains: []string{
				"python3 -m unittest discover tests",
				"agent-bridge signal QA_PASSED true",
			},
		},
		{
			name:   "Loop Breaker - Clean Tree",
			prompt: "ROLE - CODING AGENT: working tree clean",
			expectContains: []string{
				"agent-bridge signal QA_PASSED true",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			response, err := agent.Send(ctx, tc.prompt)
			if err != nil {
				t.Fatalf("Send failed: %v", err)
			}

			for _, expect := range tc.expectContains {
				if !strings.Contains(response, expect) {
					t.Errorf("Response missing expected string '%s'.\nGot:\n%s", expect, response)
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
