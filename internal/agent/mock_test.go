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

func TestMockAgent_Heuristics(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	// 1. Technical Program Manager (TPM)
	// Expects JSON list of tickets
	prompt := "You are a Technical Program Manager. Create a plan for..."
	resp, err := agent.Send(ctx, prompt)
	if err != nil {
		t.Fatalf("TPM Send failed: %v", err)
	}
	if !strings.Contains(resp, `"type": "Task"`) {
		t.Errorf("TPM heuristic failed, expected JSON with 'Task', got: %s", resp)
	}

	// 2. Coding Agent (ID:[PRIMES])
	// Expects python script implementation
	prompt = "Implement ID:[PRIMES]. create primes.py"
	resp, err = agent.Send(ctx, prompt)
	if err != nil {
		t.Fatalf("Coding Send failed: %v", err)
	}
	if !strings.Contains(resp, "cat <<EOF > primes.py") {
		t.Errorf("Coding heuristic failed, expected primes.py script, got: %s", resp)
	}

	// 3. Initializer Agent with ID:[PRIMES] (Should NOT return python script)
	// This reproduces the bug where Coding heuristic hijacked Initializer
	prompt = "You are the INITIALIZER AGENT. Please create a plan for ID:[PRIMES]..."
	resp, err = agent.Send(ctx, prompt)
	if err != nil {
		t.Fatalf("Initializer Send failed: %v", err)
	}
	if strings.Contains(resp, "cat <<EOF > primes.py") {
		t.Errorf("Initializer heuristic failed: hijacked by Coding heuristic (primes.py script returned)")
	}

	// 4. Initializer Agent (Should return feature_list.json script)
	// This verifies the enhancement/fix
	if !strings.Contains(resp, "agent-bridge import") {
		t.Errorf("Initializer heuristic failed: expected agent-bridge import script, got: %s", resp)
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
