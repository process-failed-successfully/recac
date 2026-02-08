package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent_Default(t *testing.T) {
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
	prompt := "You are the Technical Program Manager. Break down the requirements."
	response, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(response, "PRIMES") {
		t.Error("Expected JSON with project name")
	}
	if !strings.Contains(response, "req-the-makefile-targets-are-implemented") {
		t.Error("Expected specific feature ID")
	}
}

func TestMockAgent_Coding(t *testing.T) {
	agent := NewMockAgent()
	prompt := "## YOUR ROLE - CODING AGENT\nTask: Implement primes.py"
	response, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(response, "cat << 'EOF' > primes.py") {
		t.Error("Expected file creation script")
	}
	if !strings.Contains(response, "agent-bridge feature set") {
		t.Error("Expected agent-bridge call")
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
