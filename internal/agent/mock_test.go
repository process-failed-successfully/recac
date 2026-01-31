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

func TestMockAgent_PrimePython(t *testing.T) {
	agent := NewMockAgent()

	// 1. Planning Trigger (ID:[PRIMES] + AppSpec)
	planningPrompt := "This contains ID:[PRIMES] and AppSpec..."
	resp, err := agent.Send(context.Background(), planningPrompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(resp, `"title": "ID:[PRIMES] Create Prime Number Script"`) {
		t.Errorf("Expected Planning JSON, got: %s", resp)
	}

	// 2. Implementation Trigger (Task:[PRIMES] or 'primes.py' + 'create')
	// Case 1: Standard
	implPrompt1 := "Task: [PRIMES] Description: create primes.py"
	resp1, err := agent.Send(context.Background(), implPrompt1)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(resp1, "cat << 'EOF' > primes.py") {
		t.Errorf("Expected Implementation Bash, got: %s", resp1)
	}
	if !strings.Contains(resp1, "git config") {
		t.Errorf("Expected git config in response, got: %s", resp1)
	}

	// Case 2: No Task ID, but has keywords (case insensitive)
	implPrompt2 := "Please cReaTe a python script called Primes.py"
	resp2, err := agent.Send(context.Background(), implPrompt2)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(resp2, "cat << 'EOF' > primes.py") {
		t.Errorf("Expected Implementation Bash (Case Insensitive), got: %s", resp2)
	}
	if !strings.Contains(resp2, "git config") {
		t.Errorf("Expected git config in response (Case Insensitive), got: %s", resp2)
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
