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

func TestMockAgent_PrimesImplementation(t *testing.T) {
	agent := NewMockAgent()
	prompt := "Implement primes.py" // Keywords to trigger implementation logic

	response, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	expectedCommands := []string{
		"cat << 'EOF' > primes.py",
		"agent-bridge feature set req-primes-py-exists passed",
		"agent-bridge feature set req-primes-json-contains-correct-p passed",
	}

	for _, cmd := range expectedCommands {
		if !strings.Contains(response, cmd) {
			t.Errorf("Response missing expected command %q. Got:\n%s", cmd, response)
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
