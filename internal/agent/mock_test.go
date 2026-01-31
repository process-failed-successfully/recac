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

func TestMockAgent_PrimesScenario_Variants(t *testing.T) {
	agent := NewMockAgent()
	// Test without explicit "primes.py" but with feature ID
	prompt := "Task: req-primes-py-is-created. Please implement."
	response, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(response, "```bash") {
		t.Error("Response should contain bash block (req-primes variant)")
	}

	// Test with project tag
	prompt2 := "Working on ticket ID:[PRIMES] summary..."
	response2, err := agent.Send(context.Background(), prompt2)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(response2, "```bash") {
		t.Error("Response should contain bash block ([PRIMES] variant)")
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

	// Test backtick sanitization
	s2 := "hello `world`"
	if truncateString(s2, 20) != "hello 'world'" {
		t.Errorf("Expected backticks replaced, got '%s'", truncateString(s2, 20))
	}
}

func TestMockAgent_TicketGeneration_Type(t *testing.T) {
	agent := NewMockAgent()
	// Simulate the prompt sent during smoke tests (contains "app_spec.txt")
	prompt := "Please generate tickets from app_spec.txt"
	response, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Simple string check to avoid importing JSON parser logic if not needed
	// The bug is that it returns "type": "Epic"
	if strings.Contains(response, `"type": "Epic"`) {
		t.Error("MockAgent generated an Epic, but Orchestrator ignores Epics. Should be Task.")
	}
	if !strings.Contains(response, `"type": "Task"`) {
		t.Error("MockAgent should generate a Task.")
	}
}

func TestMockAgent_DefaultResponse_HasNoOp(t *testing.T) {
	agent := NewMockAgent()
	prompt := "Just a chat message"
	response, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(response, "# no-op") {
		t.Error("Default response should contain '# no-op' to prevent circuit breaker trips")
	}
}
