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

	// 1. Test TPM
	resp, err := agent.Send(ctx, "You are a Technical Program Manager")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resp, "Implement Prime Number Script") {
		t.Errorf("TPM heuristic failed, got: %s", resp)
	}

	// 2. Test Coding Agent
	resp, err = agent.Send(ctx, "YOUR ROLE - CODING AGENT")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resp, "def is_prime(n):") {
		t.Errorf("Coding Agent heuristic failed, got: %s", resp)
	}

	// 3. Test QA Agent
	resp, err = agent.Send(ctx, "YOUR ROLE - QA AGENT")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resp, "python3 test_primes.py") {
		t.Errorf("QA Agent heuristic failed, got: %s", resp)
	}

	// 4. Test Initializer
	resp, err = agent.Send(ctx, "CREATE FEATURE_LIST.JSON")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resp, "feature_list.json") {
		t.Errorf("Initializer heuristic failed, got: %s", resp)
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
