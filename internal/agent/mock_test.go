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

func TestMockAgent_PrimesHeuristic_Stateful(t *testing.T) {
	agent := NewMockAgent()

	// 1. First call: Implement
	prompt := "Implement Prime Number Generator (Task: TASK-1)"
	response, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(response, "cat <<EOF > primes.py") {
		t.Errorf("Expected primes.py creation on first call, got: %s", response)
	}

	// 2. Second call: Implement (Should mark done)
	response, err = agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(response, "agent-bridge feature set TASK-1 --status done") {
		t.Errorf("Expected completion signal on second call, got: %s", response)
	}

	// 3. First call: Tests
	prompt = "Write Unit Tests for Primes (Task: TASK-2)"
	response, err = agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(response, "cat <<EOF > test_primes.py") {
		t.Errorf("Expected test_primes.py creation on first call, got: %s", response)
	}

	// 4. Second call: Tests (Should mark done)
	response, err = agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(response, "agent-bridge feature set TASK-2 --status done") {
		t.Errorf("Expected completion signal on second call, got: %s", response)
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
