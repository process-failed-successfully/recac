package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent(t *testing.T) {
	agent := NewMockAgent()

	// 1. Default behavior
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

	// 2. TPM Heuristic
	tpmPrompt := "You are a Technical Program Manager"
	tpmResponse, err := agent.Send(context.Background(), tpmPrompt)
	if err != nil {
		t.Fatalf("TPM Send failed: %v", err)
	}
	if !strings.Contains(tpmResponse, "Implement prime number generator") {
		t.Errorf("TPM Response missing expected content, got: %s", tpmResponse)
	}

	// 3. Primes Heuristic
	primesPrompt := "ID:[PRIMES]"
	primesResponse, err := agent.Send(context.Background(), primesPrompt)
	if err != nil {
		t.Fatalf("Primes Send failed: %v", err)
	}
	if !strings.Contains(primesResponse, "```bash") {
		t.Errorf("Primes Response missing bash block, got: %s", primesResponse)
	}
	if !strings.Contains(primesResponse, "primes.py") {
		t.Errorf("Primes Response missing script content, got: %s", primesResponse)
	}

	// 4. QA Heuristic
	qaPrompt := "QA Agent check this"
	qaResponse, err := agent.Send(context.Background(), qaPrompt)
	if err != nil {
		t.Fatalf("QA Send failed: %v", err)
	}
	if qaResponse != "All tests passed" {
		t.Errorf("QA Response incorrect, got: %s", qaResponse)
	}

	// 5. Reviewer Heuristic
	revPrompt := "Reviewer please check"
	revResponse, err := agent.Send(context.Background(), revPrompt)
	if err != nil {
		t.Fatalf("Reviewer Send failed: %v", err)
	}
	if revResponse != "LGTM" {
		t.Errorf("Reviewer Response incorrect, got: %s", revResponse)
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
