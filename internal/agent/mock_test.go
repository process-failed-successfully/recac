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

	// 1. Test Ticket Generation (Pure Planning)
	promptPlan := "You are an expert Technical Program Manager (TPM). Generate tickets for a project."
	responsePlan, err := agent.Send(context.Background(), promptPlan)
	if err != nil {
		t.Fatalf("Plan Send failed: %v", err)
	}
	// Verify it returns JSON (basic check)
	if !strings.Contains(responsePlan, "[") || !strings.Contains(responsePlan, "title") {
		t.Errorf("Expected JSON ticket list, got: %s", responsePlan)
	}

	// 2. Test Coding Task (Pure Coding)
	promptCode := "Please implement a prime number generator in Python."
	responseCode, err := agent.Send(context.Background(), promptCode)
	if err != nil {
		t.Fatalf("Code Send failed: %v", err)
	}
	// Verify it returns code block
	if !strings.Contains(responseCode, "def is_prime(n):") {
		t.Errorf("Expected Python code for primes, got: %s", responseCode)
	}

	// 3. Test Coding Task (Mixed Phase / Collision)
	// This simulates the E2E prompt where history contains TPM content
	promptCollision := `## YOUR ROLE - CODING AGENT

	### RECENT HISTORY
	User: You are an expert Technical Program Manager. Create tickets.
	Agent: [{"title": "ID:[PRIMES] Implement prime number generator"}]

	### YOUR ASSIGNED TASK
	ID:[PRIMES] Implement prime number generator
	`
	responseCollision, err := agent.Send(context.Background(), promptCollision)
	if err != nil {
		t.Fatalf("Collision Send failed: %v", err)
	}
	// Verify it returns CODE, not JSON
	if strings.Contains(responseCollision, "[") && strings.Contains(responseCollision, "title") && !strings.Contains(responseCollision, "def is_prime(n):") {
		t.Errorf("Collision Test Failed: Returned JSON instead of Code. Response: %s", responseCollision)
	}
	if !strings.Contains(responseCollision, "def is_prime(n):") {
		t.Errorf("Collision Test Failed: Did not return Python code. Response: %s", responseCollision)
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
