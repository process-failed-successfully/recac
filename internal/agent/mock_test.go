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

func TestMockAgent_PrimesScenario(t *testing.T) {
	agent := NewMockAgent()
	prompt := "Please implement a python script named 'primes.py'"
	response, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(response, "```bash") {
		t.Error("Response should contain bash block")
	}
	if !strings.Contains(response, "primes.py") {
		t.Error("Response should contain primes.py")
	}
	if !strings.Contains(response, "json.dump") {
		t.Error("Response should contain json logic")
	}
}

func TestMockAgent_LoopBreaking(t *testing.T) {
	agent := NewMockAgent()
	prompt := "Previous command output: nothing to commit, working tree clean"
	response, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(response, "agent-bridge signal COMPLETED true") {
		t.Errorf("Expected loop breaking command, got: %s", response)
	}
}

func TestMockAgent_QAScenario(t *testing.T) {
	agent := NewMockAgent()
	prompt := "## YOUR ROLE - QA AGENT\n\nYour job is to verify the project."
	response, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(response, "agent-bridge signal QA_PASSED true") {
		t.Errorf("Expected QA pass signal, got: %s", response)
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
