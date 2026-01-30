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
	prompt := "Please generate tickets for the ID:[PRIMES] spec."
	response, err := agent.Send(context.Background(), prompt)

	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(response, "ID:[PRIMES]") {
		t.Errorf("Expected JSON response containing 'ID:[PRIMES]', got: %s", response)
	}
	if !strings.Contains(response, "primes.py") {
		t.Errorf("Expected JSON response containing 'primes.py', got: %s", response)
	}
}

func TestMockAgent_PrimesImplementation(t *testing.T) {
	agent := NewMockAgent()
	prompt := "Implement a python script named 'primes.py' that outputs json"
	response, err := agent.Send(context.Background(), prompt)

	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(response, "cat << 'EOF' > primes.py") {
		t.Errorf("Expected bash script solution, got: %s", response)
	}
	if !strings.Contains(response, "python3 primes.py") {
		t.Errorf("Expected python execution command, got: %s", response)
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
