package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent_TicketGeneration(t *testing.T) {
	agent := NewMockAgent()

	// Test Planner Phase
	plannerPrompt := "ID:[PRIMES] Please create a plan based on the AppSpec."
	resp, err := agent.Send(context.Background(), plannerPrompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(resp, "primes.py") {
		t.Errorf("Expected response to contain 'primes.py', got: %s", resp)
	}
	if !strings.Contains(resp, "[") || !strings.Contains(resp, "]") {
		t.Errorf("Expected response to be JSON array, got: %s", resp)
	}
}

func TestMockAgent_CodeExecution(t *testing.T) {
	agent := NewMockAgent()

	// Test Execution Phase
	// Note: Must explicitly NOT contain "AppSpec" to avoid triggering planner
	execPrompt := "Task: [PRIMES] Implement the prime number script."
	resp, err := agent.Send(context.Background(), execPrompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(resp, "cat << 'EOF' > primes.py") {
		t.Errorf("Expected bash block to create primes.py, got: %s", resp)
	}
}

func TestMockAgent_Initializer(t *testing.T) {
	agent := NewMockAgent()

	// Test Initializer Phase
	initPrompt := "You are initializing the project. Please create feature_list.json and run agent-bridge import. Context: prime-python"
	resp, err := agent.Send(context.Background(), initPrompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(resp, "agent-bridge import feature_list.json") {
		t.Errorf("Expected agent-bridge import command, got: %s", resp)
	}
	if !strings.Contains(resp, "Calculate prime numbers") {
		t.Errorf("Expected feature description, got: %s", resp)
	}
}

func TestMockAgent_DefaultResponse(t *testing.T) {
	agent := NewMockAgent()

	// Test Default
	resp, err := agent.Send(context.Background(), "Hello world")
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(resp, "Mock agent response") {
		t.Errorf("Expected default mock response, got: %s", resp)
	}
}
