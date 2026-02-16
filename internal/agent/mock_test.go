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

func TestMockAgent_Send_Heuristics(t *testing.T) {
	agent := NewMockAgent()

	// 1. Test Ticket Generation
	promptPlan := "You are an expert Technical Program Manager (TPM). Generate tickets for a project."
	responsePlan, err := agent.Send(context.Background(), promptPlan)
	if err != nil {
		t.Fatalf("Plan Send failed: %v", err)
	}
	// Verify it returns JSON (basic check)
	if !strings.Contains(responsePlan, "[") || !strings.Contains(responsePlan, "summary") {
		t.Errorf("Expected JSON ticket list, got: %s", responsePlan)
	}

	// 2. Test Coding Task
	promptCode := "Please implement a prime number generator in Python."
	responseCode, err := agent.Send(context.Background(), promptCode)
	if err != nil {
		t.Fatalf("Code Send failed: %v", err)
	}
	// Verify it returns code block
	if !strings.Contains(responseCode, "def is_prime(n):") {
		t.Errorf("Expected Python code for primes, got: %s", responseCode)
	}
}

func TestMockAgent_Send_Heuristics_Ambiguous(t *testing.T) {
	agent := NewMockAgent()

	// Ambiguous prompt: Contains "CODING AGENT" (execution role) AND "TPM" (planning keyword) AND "prime" (task keyword)
	promptAmbiguous := "## YOUR ROLE - CODING AGENT\n\nContext: previous output from TPM mentioned tickets.\nTask: Implement prime number generator."

	response, err := agent.Send(context.Background(), promptAmbiguous)
	if err != nil {
		t.Fatalf("Ambiguous Send failed: %v", err)
	}

	// It should return the Python code (execution), NOT the JSON tickets (planning)
	if !strings.Contains(response, "def is_prime(n):") {
		t.Errorf("Expected Python code for ambiguous prompt, but likely got JSON. Response: %s", response)
	}
	if strings.Contains(response, "\"summary\": \"Implement prime number generator\"") {
		t.Errorf("Got JSON ticket list instead of code execution response")
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
