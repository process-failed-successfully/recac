package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent_Send_Primes(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	// 1. Initializer
	resp, err := agent.Send(ctx, "## YOUR ROLE - INITIALIZER\nCreate a plan...")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(resp, "Calculate Primes") {
		t.Errorf("Expected Initializer response to contain 'Calculate Primes', got: %s", resp)
	}

	// 2. Coding Agent
	resp, err = agent.Send(ctx, "Create a python script named 'primes.py'")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(resp, "cat << 'EOF' > primes.py") {
		t.Errorf("Expected bash block to create primes.py, got: %s", resp)
	}
	if !strings.Contains(resp, "import json") {
		t.Errorf("Expected python script to import json, got: %s", resp)
	}
	if !strings.Contains(resp, "python3 primes.py") {
		t.Errorf("Expected command to run primes.py, got: %s", resp)
	}

	// 3. Project Manager
	resp, err = agent.Send(ctx, "## YOUR ROLE - PROJECT MANAGER\nPlease review the code...")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(resp, "APPROVED") {
		t.Errorf("Expected Project Manager approval, got: %s", resp)
	}
}
