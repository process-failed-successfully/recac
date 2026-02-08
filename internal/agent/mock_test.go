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

func TestMockAgent_PrimesScenario(t *testing.T) {
	agent := NewMockAgent()

	// 1. Initializer
	prompt := "ROLE - INITIALIZER AGENT\nAnalyze the request..."
	resp, _ := agent.Send(context.Background(), prompt)
	if !strings.Contains(resp, "agent-bridge import") {
		t.Errorf("Initializer should return agent-bridge import, got: %s", resp)
	}

	// 2. TPM
	prompt = "You are the Technical Program Manager. Break down the task [PRIMES]..."
	resp, _ = agent.Send(context.Background(), prompt)
	if !strings.Contains(resp, "ID:[PRIMES]") {
		t.Errorf("TPM should return ticket with ID:[PRIMES], got: %s", resp)
	}

	// 3. Coding
	prompt = "Implement the feature ID:[PRIMES]. Create primes.py..."
	resp, _ = agent.Send(context.Background(), prompt)
	if !strings.Contains(resp, "cat << 'EOF' > primes.py") {
		t.Errorf("Coding agent should create primes.py, got: %s", resp)
	}
	if !strings.Contains(resp, "python3 primes.py") {
		t.Errorf("Coding agent should run primes.py, got: %s", resp)
	}
	if !strings.Contains(resp, "agent-bridge feature set") {
		t.Errorf("Coding agent should mark feature set, got: %s", resp)
	}

	// 4. QA
	prompt = "You are the QA AGENT. Verify the project..."
	resp, _ = agent.Send(context.Background(), prompt)
	if !strings.Contains(resp, "agent-bridge signal QA_PASSED true") {
		t.Errorf("QA agent should signal pass, got: %s", resp)
	}
}
