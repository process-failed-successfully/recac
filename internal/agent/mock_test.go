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

func TestMockAgent_ScenarioLogic(t *testing.T) {
	agent := NewMockAgent()

	// 1. TPM Phase
	tpmPrompt := "You are an expert Technical Program Manager. Read app_spec.txt..."
	tpmResp, err := agent.Send(context.Background(), tpmPrompt)
	if err != nil {
		t.Fatalf("TPM Send failed: %v", err)
	}
	if !strings.Contains(tpmResp, "ID:[PRIMES]") {
		t.Errorf("TPM response missing ID:[PRIMES], got: %s", tpmResp)
	}
	if !strings.Contains(tpmResp, `"type": "Story"`) {
		t.Errorf("TPM response missing JSON structure, got: %s", tpmResp)
	}

	// 2. Coding Phase
	codingPrompt := "Implement the ticket ID:[PRIMES]"
	codingResp, err := agent.Send(context.Background(), codingPrompt)
	if err != nil {
		t.Fatalf("Coding Send failed: %v", err)
	}
	if !strings.Contains(codingResp, "primes.py") {
		t.Errorf("Coding response missing python file creation, got: %s", codingResp)
	}
	if !strings.Contains(codingResp, "unittest.main()") {
		t.Errorf("Coding response missing unit tests, got: %s", codingResp)
	}
}

func TestNewAgent_MockProvider(t *testing.T) {
	ag, err := NewAgent("mock", "", "", "", "")
	if err != nil {
		t.Fatalf("NewAgent('mock') failed: %v", err)
	}
	if _, ok := ag.(*MockAgent); !ok {
		t.Errorf("NewAgent('mock') returned %T, expected *MockAgent", ag)
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
