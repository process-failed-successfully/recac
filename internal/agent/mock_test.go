package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent_Send_Heuristics(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	// 1. Test Project Manager / Planning Phase
	// This prompt contains "role - project manager" AND "prime number script"
	// It should return the JSON plan, NOT the bash script
	planningPrompt := "You are an AI assistant. Your role - Project Manager. Create a plan for a prime number script."
	resp, err := agent.Send(ctx, planningPrompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(resp, "\"tickets\": [") {
		t.Errorf("Expected JSON plan (tickets) for planning prompt, got:\n%s", resp)
	}
	if strings.Contains(resp, "cat <<EOF > primes.py") {
		t.Errorf("Response contained bash script, which should be suppressed for planning prompt")
	}

	// 2. Test Coding Agent Phase
	// This prompt contains "prime" but NOT "role - project manager"
	// It should return the bash script
	codingPrompt := "Implement a prime number script."
	resp, err = agent.Send(ctx, codingPrompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(resp, "cat <<EOF > primes.py") {
		t.Errorf("Expected bash script for coding prompt, got:\n%s", resp)
	}
	if strings.Contains(resp, "\"tickets\": [") {
		t.Errorf("Response contained JSON tickets, which should be suppressed for coding prompt")
	}

	// 3. Test QA / Sign-off
	qaPrompt := "Here is the QA Report. Please review."
	resp, err = agent.Send(ctx, qaPrompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(resp, "Based on the QA Report, I approve") {
		t.Errorf("Expected approval for QA prompt, got:\n%s", resp)
	}
}
