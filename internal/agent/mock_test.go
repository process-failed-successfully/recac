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

func TestMockAgent_DefaultResponse_HasNoOp(t *testing.T) {
	agent := NewMockAgent()
	prompt := "Generic prompt not matching any triggers"
	response, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(response, "```bash") {
		t.Errorf("Default response missing bash block start")
	}
	if !strings.Contains(response, "# no-op") {
		t.Errorf("Default response missing no-op comment")
	}
	if !strings.Contains(response, "echo 'mock agent alive'") {
		t.Errorf("Default response missing echo command")
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

func TestMockAgent_Initializer(t *testing.T) {
	agent := NewMockAgent()
	prompt := "You are the Initializer Agent. Create feature_list.json based on [GEN] Create Prime Number Script ID:[PRIMES] JSON format"

	response, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(response, "agent-bridge import") {
		t.Errorf("Initializer response missing agent-bridge import")
	}
	if !strings.Contains(response, "feature_list.json") && !strings.Contains(response, "req-primes") {
		t.Errorf("Initializer response missing feature list content")
	}
	if !strings.Contains(response, "```bash") {
		t.Errorf("Initializer response missing bash block")
	}
}

func TestMockAgent_Implementation(t *testing.T) {
	agent := NewMockAgent()
	prompt := "Implement the requirements. Spec: PRIMES. Create primes.py."

	response, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(response, "primes.py") {
		t.Errorf("Implementation response missing primes.py creation")
	}
	if !strings.Contains(response, "agent-bridge update") {
		t.Errorf("Implementation response missing feature update")
	}
	if !strings.Contains(response, "req-primes") {
		t.Errorf("Implementation response missing correct feature ID")
	}
}
