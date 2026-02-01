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

func TestTruncateString(t *testing.T) {
	s := "hello world"
	if truncateString(s, 5) != "hello" {
		t.Errorf("Expected 'hello', got '%s'", truncateString(s, 5))
	}
	if truncateString(s, 20) != "hello world" {
		t.Errorf("Expected 'hello world', got '%s'", truncateString(s, 20))
	}
}

func TestMockAgent_TPM(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	// 1. Test Generic TPM Prompt
	tpmPrompt := "You are an expert Technical Program Manager... Application Specification: ..."
	resp, err := agent.Send(ctx, tpmPrompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(resp, "ID:[MOCK-1]") {
		t.Errorf("Expected generic mock ticket response, got: %s", resp)
	}
	if !strings.HasPrefix(strings.TrimSpace(resp), "[") {
		t.Errorf("Expected JSON array response, got: %s", resp)
	}

	// 2. Test Primes Scenario
	primesPrompt := "You are an expert Technical Program Manager... Application Specification: ... ID:[PRIMES] ..."
	resp, err = agent.Send(ctx, primesPrompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(resp, "ID:[PRIMES]") {
		t.Errorf("Expected PRIMES ticket response, got: %s", resp)
	}
	if !strings.Contains(resp, "primes.py") {
		t.Errorf("Expected reference to primes.py, got: %s", resp)
	}
}

func TestMockAgent_SmokeTestFlow(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	// 1. Initializer
	initPrompt := "You are the INITIALIZER AGENT..."
	initResp, err := agent.Send(ctx, initPrompt)
	if err != nil {
		t.Fatalf("Init Send failed: %v", err)
	}
	if !strings.Contains(initResp, "feature_list.json") {
		t.Errorf("Expected creation of feature_list.json, got: %s", initResp)
	}
	if !strings.Contains(initResp, "req-primes-py-exists") {
		t.Errorf("Expected prime requirement, got: %s", initResp)
	}

	// 2. Implementation
	implPrompt := "Please implement primes.py..."
	implResp, err := agent.Send(ctx, implPrompt)
	if err != nil {
		t.Fatalf("Impl Send failed: %v", err)
	}
	if !strings.Contains(implResp, "def is_prime(n):") {
		t.Errorf("Expected python implementation, got: %s", implResp)
	}
	if !strings.Contains(implResp, "agent-bridge feature set") {
		t.Errorf("Expected agent-bridge feature update, got: %s", implResp)
	}
}
