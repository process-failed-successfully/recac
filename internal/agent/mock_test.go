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

func TestMockAgent_Phases(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	// 1. TPM Phase
	prompt := "You are an expert Technical Program Manager..."
	resp, err := agent.Send(ctx, prompt)
	if err != nil {
		t.Fatalf("TPM Send failed: %v", err)
	}
	if !strings.Contains(resp, `"summary": "Implement prime number generator"`) {
		t.Errorf("TPM response invalid: %s", resp)
	}

	// 2. Coding Phase
	prompt = "Implement the prime number generator"
	resp, err = agent.Send(ctx, prompt)
	if err != nil {
		t.Fatalf("Coding Send failed: %v", err)
	}
	if !strings.Contains(resp, "cat <<EOF > primes.py") {
		t.Errorf("Coding response invalid: %s", resp)
	}

	// 3. Testing Phase
	prompt = "ran 2 tests... ok"
	resp, err = agent.Send(ctx, prompt)
	if err != nil {
		t.Fatalf("Testing Send failed: %v", err)
	}
	if !strings.Contains(resp, "Task completed") {
		t.Errorf("Testing response invalid: %s", resp)
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
