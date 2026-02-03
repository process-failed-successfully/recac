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
	prompt := "CRITICAL INSTRUCTION FOR TICKET GENERATION: Create tickets..."
	response, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.HasPrefix(strings.TrimSpace(response), "[") {
		t.Errorf("Expected JSON list response, got: %s", response)
	}
	if !strings.Contains(response, "[PRIMES]") {
		t.Errorf("Expected [PRIMES] in response, got: %s", response)
	}
}

func TestMockAgent_Implementation(t *testing.T) {
	agent := NewMockAgent()
	prompt := "Please implement primes.py script."
	response, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(response, "cat << 'EOF' > primes.py") {
		t.Errorf("Expected python script creation, got: %s", response)
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
