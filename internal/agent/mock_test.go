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
}

func TestMockAgent_LifecycleRoles(t *testing.T) {
	agent := NewMockAgent()

	// 1. QA Agent
	qaPrompt := "## YOUR ROLE - QA AGENT. Verify the project."
	qaResponse, err := agent.Send(context.Background(), qaPrompt)
	if err != nil {
		t.Fatalf("QA Send failed: %v", err)
	}
	if !strings.Contains(qaResponse, "agent-bridge signal set QA_PASSED true") {
		t.Errorf("QA response should set QA_PASSED signal, got: %s", qaResponse)
	}

	// 2. Manager Agent
	mgrPrompt := "Manager Review. Please approve."
	mgrResponse, err := agent.Send(context.Background(), mgrPrompt)
	if err != nil {
		t.Fatalf("Manager Send failed: %v", err)
	}
	if !strings.Contains(mgrResponse, "agent-bridge signal set PROJECT_SIGNED_OFF true") {
		t.Errorf("Manager response should set PROJECT_SIGNED_OFF signal, got: %s", mgrResponse)
	}

	// 3. Completion
	donePrompt := "All features are marked as done/passing."
	doneResponse, err := agent.Send(context.Background(), donePrompt)
	if err != nil {
		t.Fatalf("Completion Send failed: %v", err)
	}
	if !strings.Contains(doneResponse, "agent-bridge signal set COMPLETED true") {
		t.Errorf("Completion response should set COMPLETED signal, got: %s", doneResponse)
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
