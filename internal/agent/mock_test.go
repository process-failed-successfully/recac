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

func TestMockAgent_TicketGeneration(t *testing.T) {
	agent := NewMockAgent()
	// Simulate the AppSpec prompt
	prompt := `### ID:[PRIMES] Prime Number Script
CRITICAL INSTRUCTION: You MUST create exactly ONE ticket. Type: Task.
...
CRITICAL INSTRUCTION FOR TICKET GENERATION:
Create a SINGLE Ticket (Task) for this work...`

	response, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(response, "\"tickets\": [") {
		t.Errorf("Expected ticket JSON response, got: %s", response)
	}
	if !strings.Contains(response, "ID:[PRIMES]") {
		t.Errorf("Expected ID:[PRIMES] in response, got: %s", response)
	}
}

func TestMockAgent_Implementation(t *testing.T) {
	agent := NewMockAgent()
	// Simulate implementation prompt (which shouldn't have the ticket generation instruction)
	prompt := `Create a python script named 'primes.py'. It MUST be python.
It must calculate all prime numbers less than 10,000...`

	response, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(response, "<file path=\"primes.py\">") {
		t.Errorf("Expected file block in response, got: %s", response)
	}
	if !strings.Contains(response, "PROJECT_SIGNED_OFF") {
		t.Errorf("Expected signoff in response, got: %s", response)
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
