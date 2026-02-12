package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent_Heuristics(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	// 1. Test TPM
	tpmPrompt := "You are an expert Technical Program Manager (TPM). ID:[PRIMES] ..."
	resp, err := agent.Send(ctx, tpmPrompt)
	if err != nil {
		t.Fatalf("TPM Send failed: %v", err)
	}
	if !strings.Contains(resp, "ID:[PRIMES]") || !strings.Contains(resp, "Task") {
		t.Errorf("TPM response invalid: %s", resp)
	}

	// 2. Test Initializer
	initPrompt := "You are an Initializer Agent ..."
	resp, err = agent.Send(ctx, initPrompt)
	if err != nil {
		t.Fatalf("Initializer Send failed: %v", err)
	}
	if !strings.Contains(resp, "Initializing repository") {
		t.Errorf("Initializer response invalid: %s", resp)
	}

	// 3. Test Coding Agent
	codingPrompt := "## YOUR ROLE - CODING AGENT ... Implement primes.py"
	resp, err = agent.Send(ctx, codingPrompt)
	if err != nil {
		t.Fatalf("Coding Send failed: %v", err)
	}
	if !strings.Contains(resp, "cat << 'EOF' > primes.py") {
		t.Errorf("Coding response invalid: %s", resp)
	}
	if !strings.Contains(resp, "with open('primes.json', 'w')") {
		t.Errorf("Coding response missing file write logic: %s", resp)
	}

	// 4. Test QA/Manager
	qaPrompt := "## YOUR ROLE - QA AGENT ... Verify code"
	resp, err = agent.Send(ctx, qaPrompt)
	if err != nil {
		t.Fatalf("QA Send failed: %v", err)
	}
	if !strings.Contains(resp, "LGTM") {
		t.Errorf("QA response invalid: %s", resp)
	}
}
