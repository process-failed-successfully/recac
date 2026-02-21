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

func TestMockAgent_PrimesScenario(t *testing.T) {
	agent := NewMockAgent()

	// 1. Coding/Implementation Prompt
	codePrompt := "Please implement the prime number script (primes.py)"
	codeResp, err := agent.Send(context.Background(), codePrompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(codeResp, "cat << 'EOF' > primes.py") {
		t.Errorf("Expected bash script for coding prompt, got: %s", codeResp)
	}

	// 2. TPM/Planning Prompt
	tpmPrompt := "You are an expert Technical Program Manager. Decompose the prime number script spec into JSON tickets. Output purely JSON."
	tpmResp, err := agent.Send(context.Background(), tpmPrompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(tpmResp, "\"title\": \"ID:[PRIMES]") {
		t.Errorf("Expected JSON ticket for TPM prompt, got: %s", tpmResp)
	}
	if strings.Contains(tpmResp, "cat << 'EOF'") {
		t.Errorf("Did NOT expect bash script for TPM prompt, got: %s", tpmResp)
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
