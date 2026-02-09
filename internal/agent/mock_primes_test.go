package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent_Primes(t *testing.T) {
	agent := NewMockAgent()
	prompt := "Please implement [PRIMES] calculation"

	resp, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(resp, "cat << 'EOF' > primes.py") {
		t.Error("Response should contain primes.py creation script")
	}
	if !strings.Contains(resp, "range(10000)") {
		t.Error("Response should calculate primes up to 10000")
	}
	if !strings.Contains(resp, "json.dump({\"primes\": primes}, f)") {
		t.Error("Response should output JSON object with 'primes' key")
	}
	if !strings.Contains(resp, "python3 primes.py") {
		t.Error("Response should run the python script")
	}
	if !strings.Contains(resp, "agent-bridge feature set") {
		t.Error("Response should mark features as passed")
	}
	if !strings.Contains(resp, "--status done --passes true") {
		t.Error("Response should use correct flags for feature status update")
	}
}

func TestMockAgent_Primes_EnvInjection(t *testing.T) {
	// Simulate environment variable injection
	t.Setenv("RECAC_INJECTED_FEATURES", `{"project_name":"ID:[PRIMES] Implement Primes Calculation"}`)

	agent := NewMockAgent()
	// Prompt DOES NOT contain [PRIMES]
	prompt := "Please implement the task for MFLP-5825"

	resp, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(resp, "cat << 'EOF' > primes.py") {
		t.Error("Response should contain primes.py creation script (triggered by env var)")
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
