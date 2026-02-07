package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent(t *testing.T) {
	agent := NewMockAgent()

	// 1. Default Behavior
	prompt := "This is a test prompt that is long enough to be truncated"
	response, err := agent.Send(context.Background(), prompt)

	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(response, "Mock response to") {
		t.Errorf("Response missing prefix, got: %s", response)
	}

	// 2. Primes Heuristic
	promptPrimes := "Please implement the [PRIMES] feature by creating primes.py"
	respPrimes, err := agent.Send(context.Background(), promptPrimes)
	if err != nil {
		t.Fatalf("Primes prompt failed: %v", err)
	}
	if !strings.Contains(respPrimes, "cat << 'EOF' > primes.py") {
		t.Errorf("Primes response missing bash script: %s", respPrimes)
	}

	// 3. Initializer Heuristic
	promptInit := "Please analyze this ticket and provide a plan feature_list.json"
	respInit, err := agent.Send(context.Background(), promptInit)
	if err != nil {
		t.Fatalf("Init prompt failed: %v", err)
	}
	if !strings.Contains(respInit, "req-primes-py-exists") {
		t.Errorf("Init response missing feature ID: %s", respInit)
	}

	// 4. TPM Heuristic
	promptTPM := "Technical Program Manager: Please generate tickets"
	respTPM, err := agent.Send(context.Background(), promptTPM)
	if err != nil {
		t.Fatalf("TPM prompt failed: %v", err)
	}
	if !strings.Contains(respTPM, "ID:[PRIMES]") {
		t.Errorf("TPM response missing ID: %s", respTPM)
	}
	if !strings.Contains(respTPM, "[") || !strings.Contains(respTPM, "]") {
		t.Errorf("TPM response is not a list: %s", respTPM)
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
