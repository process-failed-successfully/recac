package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent_Scenario_PrimePython(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	// 1. Test Ticket Generation Prompt (Initializer)
	promptPlan := "Generate tickets... ID:[PRIMES] ..."
	respPlan, err := agent.Send(ctx, promptPlan)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(respPlan, "agent-bridge import") {
		t.Errorf("Expected response to contain 'agent-bridge import', got: %s", respPlan)
	}
	if !strings.Contains(respPlan, `"id": "PRIMES"`) {
		t.Errorf("Expected response to contain feature ID 'PRIMES', got: %s", respPlan)
	}
	if !strings.Contains(respPlan, "cat << 'EOF' | agent-bridge import") {
		t.Errorf("Expected response to be a bash block piping to agent-bridge, got: %s", respPlan)
	}

	// 2. Test Execution Prompt (Coding Agent)
	promptExec := "Implement primes.py and primes.json..."
	respExec, err := agent.Send(ctx, promptExec)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(respExec, "cat << 'EOF' > primes.py") {
		t.Errorf("Expected response to contain bash command to create primes.py, got: %s", respExec)
	}
	if !strings.Contains(respExec, "python3 primes.py") {
		t.Errorf("Expected response to contain command to run script, got: %s", respExec)
	}
	// Check for correct prime logic snippet (basic check)
	if !strings.Contains(respExec, "def is_prime(n):") {
		t.Errorf("Expected response to contain python code, got: %s", respExec)
	}
	// Check for completion signal
	if !strings.Contains(respExec, "agent-bridge feature set PRIMES --status done --passes true") {
		t.Errorf("Expected response to mark feature as done, got: %s", respExec)
	}
}
