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

func TestTruncateString(t *testing.T) {
	s := "hello world"
	if truncateString(s, 5) != "hello" {
		t.Errorf("Expected 'hello', got '%s'", truncateString(s, 5))
	}
	if truncateString(s, 20) != "hello world" {
		t.Errorf("Expected 'hello world', got '%s'", truncateString(s, 20))
	}
}

func TestMockAgent_TicketGenerationWithPrimesContent(t *testing.T) {
	agent := NewMockAgent()
	// Simulate the prompt sent during ticket generation for the primes scenario
	prompt := "app_spec.txt: Implement a python script named 'primes.py'..."
	response, err := agent.Send(context.Background(), prompt)

	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Verify we get the specific task, not the generic Epic
	if !strings.Contains(response, "\"type\": \"Task\"") {
		t.Error("Expected Type: Task for primes scenario")
	}
	if !strings.Contains(response, "Implement a python script named 'primes.py'") {
		t.Error("Expected specific description containing 'primes.py'")
	}
	if strings.Contains(response, "Mock epic") {
		t.Error("Should not return generic mock epic")
	}
}
