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

func TestMockAgent_Initializer(t *testing.T) {
	agent := NewMockAgent()
	prompt := "ROLE - INITIALIZER AGENT\nRepo: https://github.com/test/repo"
	response, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(response, "git init") {
		t.Error("Expected 'git init' in response")
	}
	if !strings.Contains(response, "git remote add origin") {
		t.Error("Expected 'git remote add origin' in response")
	}
}

func TestMockAgent_Coding(t *testing.T) {
	agent := NewMockAgent()
	prompt := "Please implement primes.py"
	response, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	expectedStrings := []string{
		"BRANCH_NAME=\"agent/${RECAC_PROJECT_ID:-PRIMES-mock}\"",
		"git checkout -B \"$BRANCH_NAME\"",
		"git push --force origin \"$BRANCH_NAME\"",
		"primes.py",
	}

	for _, exp := range expectedStrings {
		if !strings.Contains(response, exp) {
			t.Errorf("Expected response to contain %q, but it didn't", exp)
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
