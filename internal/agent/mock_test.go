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

	// 1. Test Planner/Architect Role
	plannerPrompt := "ROLE: Lead Software Architect. CRITICAL INSTRUCTION FOR TICKET GENERATION. Please create a plan for primes.py [PRIMES]"
	planResponse, err := agent.Send(context.Background(), plannerPrompt)
	if err != nil {
		t.Fatalf("Planner Send failed: %v", err)
	}
	if !strings.Contains(planResponse, `"project_name": "Prime Number Script"`) {
		t.Errorf("Planner response should contain JSON plan, got: %s", planResponse)
	}

	// 2. Test TPM Role
	tpmPrompt := "You are an expert Technical Program Manager (TPM). [PRIMES]"
	tpmResponse, err := agent.Send(context.Background(), tpmPrompt)
	if err != nil {
		t.Fatalf("TPM Send failed: %v", err)
	}
	if !strings.Contains(tpmResponse, `"title": "ID:[PRIMES] Prime Number Script"`) {
		t.Errorf("TPM response should contain JSON list, got: %s", tpmResponse)
	}

	// 3. Test Coding Agent Role
	coderPrompt := "YOUR ROLE - CODING AGENT. Implement feature [PRIMES] for primes.py"
	coderResponse, err := agent.Send(context.Background(), coderPrompt)
	if err != nil {
		t.Fatalf("Coder Send failed: %v", err)
	}
	if !strings.Contains(coderResponse, "git config user.email") {
		t.Errorf("Coder response should contain bash script with git config, got: %s", coderResponse)
	}

	// 4. Test Coding Agent Role with Feature ID (Robustness Check)
	// This simulates a truncated prompt or one where [PRIMES] is missing but the feature ID is present
	featurePrompt := "YOUR ROLE - CODING AGENT. Task: req-implement-prime-calculation-lo. Please implement."
	featureResponse, err := agent.Send(context.Background(), featurePrompt)
	if err != nil {
		t.Fatalf("Feature ID Send failed: %v", err)
	}
	if !strings.Contains(featureResponse, "git config user.email") {
		t.Errorf("Feature ID response should contain bash script with git config, got: %s", featureResponse)
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
