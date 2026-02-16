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

func TestMockAgent_PrimesHeuristic_TPM(t *testing.T) {
	agent := NewMockAgent()

	prompt := "## YOUR ROLE - TECHNICAL PROGRAM MANAGER\nID:[PRIMES] Prime Number Script"
	response, err := agent.Send(context.Background(), prompt)

	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(response, "ID:[PRIMES] Create Prime Number Script") {
		t.Errorf("Response missing expected JSON content, got: %s", response)
	}
	if !strings.Contains(response, "\"type\": \"Task\"") {
		t.Errorf("Response missing expected JSON type, got: %s", response)
	}
}

func TestMockAgent_PrimesHeuristic_Coding(t *testing.T) {
	agent := NewMockAgent()

	prompt := "## YOUR ROLE - CODING AGENT\nFeature ID: PROJ-123\nID:[PRIMES] Prime Number Script"
	response, err := agent.Send(context.Background(), prompt)

	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(response, "cat << 'EOF' > primes.py") {
		t.Errorf("Response missing bash block for file creation, got: %s", response)
	}
	if !strings.Contains(response, "agent-bridge feature set PROJ-123") {
		t.Errorf("Response missing feature set command with correct ID, got: %s", response)
	}
	if !strings.Contains(response, "agent-bridge signal COMPLETED true") {
		t.Errorf("Response missing completion signal, got: %s", response)
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
