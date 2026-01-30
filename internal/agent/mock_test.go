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

func TestTruncateString(t *testing.T) {
	s := "hello world"
	if truncateString(s, 5) != "hello" {
		t.Errorf("Expected 'hello', got '%s'", truncateString(s, 5))
	}
	if truncateString(s, 20) != "hello world" {
		t.Errorf("Expected 'hello world', got '%s'", truncateString(s, 20))
	}
}

func TestMockAgent_SmartLogic(t *testing.T) {
	agent := NewMockAgent()

	// 1. Test Ticket Generation
	promptTicket := "### ID:[PRIMES] Prime Number Script\nCRITICAL INSTRUCTION: You MUST create exactly ONE ticket. Type: Task."
	resp, err := agent.Send(context.Background(), promptTicket)
	if err != nil {
		t.Fatalf("Failed to send ticket prompt: %v", err)
	}
	if !strings.Contains(resp, `"id": "PRIMES"`) {
		t.Errorf("Expected ticket JSON, got: %s", resp)
	}

	// 2. Test Implementation
	promptImpl := "Create a python script named 'primes.py'"
	resp, err = agent.Send(context.Background(), promptImpl)
	if err != nil {
		t.Fatalf("Failed to send implementation prompt: %v", err)
	}
	if !strings.Contains(resp, "cat << 'EOF' > primes.py") {
		t.Errorf("Expected bash script for primes.py, got: %s", resp)
	}
	if !strings.Contains(resp, "cat << 'EOF' > primes.json") {
		t.Errorf("Expected bash script for primes.json, got: %s", resp)
	}
	// Check for a few primes to ensure json is there
	if !strings.Contains(resp, `"primes": [2, 3, 5, 7`) {
		t.Errorf("Expected primes data, got: %s", resp)
	}
}

func TestNewAgent_Mock(t *testing.T) {
	agent, err := NewAgent("mock", "", "", "", "")
	if err != nil {
		t.Fatalf("Failed to create mock agent: %v", err)
	}
	if _, ok := agent.(*MockAgent); !ok {
		t.Errorf("Expected *MockAgent, got %T", agent)
	}
}
