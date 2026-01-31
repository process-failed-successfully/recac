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

	// 1. Prime Numbers (Exact)
	resp, _ := agent.Send(context.Background(), "Task: PRIMES - Implement it")
	if !strings.Contains(resp, "cat <<EOF > primes.py") {
		t.Errorf("PRIMES task didn't trigger bash script")
	}

	// 2. Prime Numbers (Lowercase)
	resp2, _ := agent.Send(context.Background(), "Implement prime numbers logic")
	if !strings.Contains(resp2, "cat <<EOF > primes.py") {
		t.Errorf("lowercase prime prompt didn't trigger bash script")
	}

	// 3. Sub-task: is_prime
	resp3, _ := agent.Send(context.Background(), "Implement is_prime function")
	if !strings.Contains(resp3, "cat <<EOF > primes.py") {
		t.Errorf("is_prime prompt didn't trigger bash script")
	}

	// 4. Sub-task: main block
	resp4, _ := agent.Send(context.Background(), "Implement main execution block")
	if !strings.Contains(resp4, "cat <<EOF > primes.py") {
		t.Errorf("main execution block prompt didn't trigger bash script")
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
