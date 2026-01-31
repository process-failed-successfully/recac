package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent_Scenario_PrimePython(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	// 1. Test Ticket Generation Prompt
	promptPlan := "Generate tickets... ID:[PRIMES] ..."
	respPlan, err := agent.Send(ctx, promptPlan)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(respPlan, "ID:[PRIMES]") {
		t.Errorf("Expected response to contain ticket ID [PRIMES], got: %s", respPlan)
	}
	if !strings.Contains(respPlan, `"type": "Task"`) {
		t.Errorf("Expected response to contain Task type, got: %s", respPlan)
	}

	// 2. Test Execution Prompt
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
}
