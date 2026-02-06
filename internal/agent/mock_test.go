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

func TestMockAgent_SmartPrimes(t *testing.T) {
	agent := NewMockAgent()

	// Test 1: Coding Agent prompts (Bash script)
	codingPrompts := []string{
		"Please implement the [PRIMES] task.",
		"Implement prime-python scenario.",
		"Create a Prime Number Script",
		"## YOUR ROLE - CODING AGENT\nImplement [PRIMES]",
	}

	for _, p := range codingPrompts {
		resp, err := agent.Send(context.Background(), p)
		if err != nil {
			t.Errorf("Send failed for prompt '%s': %v", p, err)
		}

		if !strings.Contains(resp, "primes.py") {
			t.Errorf("Response for '%s' should contain primes.py", p)
		}
		if !strings.Contains(resp, "cat << 'EOF' > primes.py") {
			t.Errorf("Response for '%s' should contain bash heredoc", p)
		}
		if !strings.Contains(resp, "git commit") {
			t.Errorf("Response for '%s' should contain git commit", p)
		}
		if !strings.Contains(resp, "agent-bridge feature set") {
			t.Errorf("Response for '%s' should contain agent-bridge feature set", p)
		}
	}

	// Test 2: Planner prompts (JSON plan)
	plannerPrompts := []string{
		"## ROLE: Lead Software Architect\nAnalyze [PRIMES]",
		"Generate feature_list.json for [PRIMES]",
	}

	for _, p := range plannerPrompts {
		resp, err := agent.Send(context.Background(), p)
		if err != nil {
			t.Errorf("Send failed for prompt '%s': %v", p, err)
		}

		if strings.Contains(resp, "cat << 'EOF'") {
			t.Errorf("Response for '%s' should NOT contain bash script", p)
		}
		if !strings.Contains(resp, "\"project_name\": \"Prime Number Script\"") {
			t.Errorf("Response for '%s' should contain JSON project name", p)
		}
		if !strings.Contains(resp, "\"steps\": [") {
			t.Errorf("Response for '%s' should contain JSON steps", p)
		}
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
