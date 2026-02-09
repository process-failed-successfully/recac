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
	if !strings.Contains(resp, "python3 primes.py") {
		t.Error("Response should run the python script")
	}
	if !strings.Contains(resp, "agent-bridge feature set") {
		t.Error("Response should mark features as passed")
	}
	if !strings.Contains(resp, "range(10000)") {
		t.Error("Response should calculate primes up to 10000")
	}
	if !strings.Contains(resp, "--status Done --passes true") {
		t.Error("Response should use correct agent-bridge syntax")
	}

	// Test case 2: "Agent" role from CI failure scenario
	// Use the feature ID which is the specific trigger we want to support
	prompt2 := "## YOUR ROLE - Agent\nTask: Implement req-script-prints-primes-up-to-100"
	resp2, err2 := agent.Send(context.Background(), prompt2)
	if err2 != nil {
		t.Fatalf("Send 2 failed: %v", err2)
	}
	if !strings.Contains(resp2, "cat << 'EOF' > primes.py") {
		t.Error("Response 2 (Agent role) should contain primes.py creation script")
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
