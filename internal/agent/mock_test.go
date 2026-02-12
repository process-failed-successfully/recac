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

func TestMockAgent_TPMHeuristics(t *testing.T) {
	agent := NewMockAgent()

	// Simulate TPM Prompt for Primes
	prompt := "You are an expert Technical Program Manager... ID:[PRIMES] ..."
	response, err := agent.Send(context.Background(), prompt)

	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(response, "ID:[PRIMES]") {
		t.Errorf("Response missing ID:[PRIMES], got: %s", response)
	}

	if !strings.Contains(response, "\"type\": \"Task\"") {
		t.Errorf("Response missing type: Task, got: %s", response)
	}
}

func TestMockAgent_CodingHeuristics(t *testing.T) {
	agent := NewMockAgent()

	// Simulate Coding Prompt for Primes
	prompt := "YOUR ROLE - CODING AGENT ... Feature ID: PRIMES ..."
	response, err := agent.Send(context.Background(), prompt)

	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(response, "cat << 'EOF' > primes.py") {
		t.Errorf("Response missing python script creation, got: %s", response)
	}

	if !strings.Contains(response, "agent-bridge feature set PRIMES") {
		t.Errorf("Response missing feature update command, got: %s", response)
	}
}

func TestMockAgent_QAHeuristics(t *testing.T) {
	agent := NewMockAgent()

	// Simulate QA Prompt
	prompt := "YOUR ROLE - QA AGENT ..."
	response, err := agent.Send(context.Background(), prompt)

	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(response, "agent-bridge signal QA_PASSED true") {
		t.Errorf("Response missing QA signal, got: %s", response)
	}
}

func TestMockAgent_ManagerHeuristics(t *testing.T) {
	agent := NewMockAgent()

	// Simulate Manager Prompt
	prompt := "YOUR ROLE - PROJECT MANAGER ..."
	response, err := agent.Send(context.Background(), prompt)

	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(response, "agent-bridge signal PROJECT_SIGNED_OFF true --privileged") {
		t.Errorf("Response missing Manager signal, got: %s", response)
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
