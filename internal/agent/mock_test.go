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
	// Test Planning Phase Heuristic
	t.Run("Planning Phase", func(t *testing.T) {
		agent := NewMockAgent()
		prompt := "You are an expert Technical Program Manager (TPM). Please decompose..."
		resp, err := agent.Send(context.Background(), prompt)
		if err != nil {
			t.Fatalf("Send failed: %v", err)
		}
		if !strings.Contains(resp, "ID:[PRIMES]") {
			t.Errorf("Expected planning response with ID:[PRIMES], got: %s", resp)
		}
	})

	// Test Execution Phase Heuristic
	t.Run("Execution Phase", func(t *testing.T) {
		agent := NewMockAgent()
		prompt := "Please write a script to calculate prime numbers and output to primes.json"

		// First call: Should return the script
		resp, err := agent.Send(context.Background(), prompt)
		if err != nil {
			t.Fatalf("Send failed: %v", err)
		}
		if !strings.Contains(resp, "def is_prime(n):") {
			t.Errorf("Expected python code in response 1, got: %s", resp)
		}
		if !strings.Contains(resp, "cat << 'EOF' > primes.py") {
			t.Errorf("Expected bash file creation block in response 1, got: %s", resp)
		}

		// Second call: Should return completion message
		resp2, err := agent.Send(context.Background(), prompt)
		if err != nil {
			t.Fatalf("Send failed: %v", err)
		}
		if strings.Contains(resp2, "cat << 'EOF' > primes.py") {
			t.Errorf("Expected completion message in response 2, got script again: %s", resp2)
		}
		if !strings.Contains(resp2, "Task is complete") && !strings.Contains(resp2, "task is complete") {
			t.Errorf("Expected completion message in response 2, got: %s", resp2)
		}
	})
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
