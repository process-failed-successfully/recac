package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent_Primes(t *testing.T) {
	agent := NewMockAgent()
	// Test case 1: Standard CODING AGENT prompt
	prompt := "## YOUR ROLE - CODING AGENT\nPlease implement [PRIMES] calculation"

	resp, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(resp, "cat << 'EOF' > primes.py") {
		t.Error("Response should contain primes.py creation script")
	}

	// Test case 2: "Agent" role from CI failure scenario
	prompt2 := "## YOUR ROLE - Agent\nTask: Implement req-script-prints-primes-up-to-100"
	resp2, err2 := agent.Send(context.Background(), prompt2)
	if err2 != nil {
		t.Fatalf("Send 2 failed: %v", err2)
	}
	if !strings.Contains(resp2, "cat << 'EOF' > primes.py") {
		t.Error("Response 2 (Agent role) should contain primes.py creation script")
	}

	// Test case 3: Feature ID ONLY (No specific role, simulating fallback or varied role name)
	prompt3 := "Task: req-script-prints-primes-up-to-100"
	resp3, err3 := agent.Send(context.Background(), prompt3)
	if err3 != nil {
		t.Fatalf("Send 3 failed: %v", err3)
	}
	if !strings.Contains(resp3, "cat << 'EOF' > primes.py") {
		t.Error("Response 3 (Feature ID only) should contain primes.py creation script")
	}
}

func TestMockAgent_GenericFallback(t *testing.T) {
	agent := NewMockAgent()
	prompt := "Hello world"

	resp, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(resp, "echo \"Mock Agent is alive\"") {
		t.Error("Response should contain dummy command to prevent NO-OP loop")
	}
}
