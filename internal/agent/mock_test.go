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
		contains string
	}{
		{"QA", "I am the QA AGENT", "agent-bridge signal QA_PASSED true"},
		{"Manager", "ROLE - PROJECT MANAGER please review", "agent-bridge signal PROJECT_SIGNED_OFF true"},
		{"Manager2", "MANAGER REVIEW required", "agent-bridge signal PROJECT_SIGNED_OFF true"},
		{"Initializer", "INITIALIZER agent start", "agent-bridge import < feature_list.json"},
		{"SetupRepo", "Please req-setup-repo", "agent-bridge feature set req-setup-repo --status done"},
		{"Primes", "Work on req-implement-primes", "python3 primes.py"},
		{"Tests", "Work on req-implement-tests", "touch test_primes.py"},
		{"Makefile", "Work on req-the-makefile-targets-are-implemented", "touch Makefile"},
		{"CI", "Work on req-ci-workflow", "touch .github/workflows/ci.yml"},
		{"LoopBreaker", "nothing to commit, tree clean", "agent-bridge signal COMPLETED true"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := agent.Send(ctx, tt.prompt)
			if err != nil {
				t.Fatalf("Send failed: %v", err)
			}
			if !strings.Contains(resp, tt.contains) {
				t.Errorf("Response for %s missing '%s', got:\n%s", tt.name, tt.contains, resp)
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
