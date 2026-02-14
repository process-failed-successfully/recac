package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent_Basics(t *testing.T) {
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

	// 1. TPM
	promptTPM := "You are the Technical Program Manager (TPM). Break down the task."
	resp, err := agent.Send(ctx, promptTPM)
	if err != nil {
		t.Fatalf("TPM Send failed: %v", err)
	}
	if !strings.Contains(resp, "[{\"title\":") {
		t.Errorf("TPM heuristic failed, got: %s", resp)
	}

	// 2. Primes - Create
	promptPrimesCreate := "Task: ID:[PRIMES] Implement Primes. Status: nothing to commit"
	resp, err = agent.Send(ctx, promptPrimesCreate)
	if err != nil {
		t.Fatalf("Primes Create Send failed: %v", err)
	}
	if !strings.Contains(resp, "cat <<EOF > primes.py") {
		t.Errorf("Primes Create heuristic failed, got: %s", resp)
	}

	// 3. Primes - Done
	promptPrimesDone := "Task: ID:[PRIMES]. Files: primes.py. Status: working tree clean"
	resp, err = agent.Send(ctx, promptPrimesDone)
	if err != nil {
		t.Fatalf("Primes Done Send failed: %v", err)
	}
	if !strings.Contains(resp, "python3 primes.py") {
		t.Errorf("Primes Done heuristic failed, got: %s", resp)
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
