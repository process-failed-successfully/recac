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

	if !strings.Contains(response, "I will implement the requested features") {
		t.Errorf("Response missing prefix, got: %s", response)
	}

	if !strings.Contains(response, "agent-bridge feature list") {
		t.Errorf("Response missing body, got: %s", response)
	}
}

func TestMockAgent_PrimesScenario(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	// 1. Test "Create primes.py" prompt
	prompt1 := "Task: Create primes.py"
	resp1, err := agent.Send(ctx, prompt1)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(resp1, "cat << 'EOF' > primes.py") {
		t.Errorf("Expected [PRIMES] logic for prompt %q, got: %s", prompt1, resp1)
	}

	// 2. Test "Implement verify_prime function" prompt
	prompt2 := "Task: Implement verify_prime function"
	resp2, err := agent.Send(ctx, prompt2)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	// This now triggers [PRIMES] because of "verify_prime" keyword
	if !strings.Contains(resp2, "cat << 'EOF' > primes.py") {
		t.Errorf("Expected [PRIMES] logic for prompt %q, got: %s", prompt2, resp2)
	}

	// 3. Test generic "Add unit tests" prompt (should hit Default)
	prompt3 := "Task: Add unit tests"
	resp3, err := agent.Send(ctx, prompt3)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if strings.Contains(resp3, "primes.py") {
		t.Errorf("Prompt %q triggered logic using primes.py (should be mock_impl.py)!", prompt3)
	}
	if !strings.Contains(resp3, "mock_impl.py") {
		t.Errorf("Expected Default logic using mock_impl.py for prompt %q, got: %s", prompt3, resp3)
	}

	// 4. Test specific "Add unit tests for primes.py" prompt (should hit [PRIMES])
	prompt4 := "Task: Add unit tests for primes.py"
	resp4, err := agent.Send(ctx, prompt4)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(resp4, "cat << 'EOF' > primes.py") {
		t.Errorf("Expected [PRIMES] logic for prompt %q, got: %s", prompt4, resp4)
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
