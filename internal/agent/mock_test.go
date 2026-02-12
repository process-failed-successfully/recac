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

	// Test 1: Coding Agent (Prime Python)
	prompt := "Create a python script to calculate primes"
	response, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(response, "cat << 'EOF' > primes.py") {
		t.Errorf("Response missing primes.py creation, got: %s", response)
	}
	if !strings.Contains(response, "agent-bridge feature set PRIMES --status completed") {
		t.Errorf("Response missing feature completion, got: %s", response)
	}

	// Test 2: Initializer Agent
	promptInit := "I am the initializer agent. Perform feature extraction."
	responseInit, err := agent.Send(context.Background(), promptInit)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(responseInit, "agent-bridge import --project") {
		t.Errorf("Response missing agent-bridge import, got: %s", responseInit)
	}

	// Test 3: Project Manager
	promptMgr := "I am the project manager. Sign off."
	responseMgr, err := agent.Send(context.Background(), promptMgr)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(responseMgr, "agent-bridge signal PROJECT_SIGNED_OFF --privileged") {
		t.Errorf("Response missing project sign off, got: %s", responseMgr)
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
