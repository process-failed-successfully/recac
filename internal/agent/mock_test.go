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

	// 4. Test Feature ID Detection (Robustness)
	// This tests that we detect the scenario even if "primes.py" is missing but the ID is present
	idPrompt := "YOUR ROLE - CODING AGENT. Task ID: req-implement-prime-calculation-lo"
	idResponse, err := agent.Send(context.Background(), idPrompt)
	if err != nil {
		t.Fatalf("Feature ID Send failed: %v", err)
	}
	if !strings.Contains(idResponse, "git config user.email") {
		t.Errorf("Response via ID detection should contain bash script, got: %s", idResponse)
	}

	// 5. Test "Nothing to Commit" Heuristic
	// This ensures we break the loop when git says nothing changed
	nothingPrompt := "YOUR ROLE - CODING AGENT. Task ID: [PRIMES]. Output: nothing to commit, working tree clean"
	nothingResponse, err := agent.Send(context.Background(), nothingPrompt)
	if err != nil {
		t.Fatalf("Nothing to Commit Send failed: %v", err)
	}
	if !strings.Contains(nothingResponse, "agent-bridge feature set") {
		t.Errorf("Response for 'nothing to commit' should contain feature set command, got: %s", nothingResponse)
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
