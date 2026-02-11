package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent_HeuristicResponses(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	// 1. TPM (Planning)
	resp, err := agent.Send(ctx, "You are an expert Technical Program Manager (TPM)...")
	if err != nil {
		t.Fatalf("TPM Send failed: %v", err)
	}
	if !strings.Contains(resp, "```json") || !strings.Contains(resp, "ID:[PRIMES]") {
		t.Errorf("TPM response invalid. Expected JSON markdown block. Got:\n%s", resp)
	}

	// 2. Initializer
	resp, err = agent.Send(ctx, "## YOUR ROLE - INITIALIZER AGENT")
	if err != nil {
		t.Fatalf("Initializer Send failed: %v", err)
	}
	if !strings.Contains(resp, "```bash") || !strings.Contains(resp, "agent-bridge import") {
		t.Errorf("Initializer response invalid. Expected bash markdown block. Got:\n%s", resp)
	}

	// 3. Coding Agent
	resp, err = agent.Send(ctx, "## YOUR ROLE - CODING AGENT\nImplement the Prime Number Script")
	if err != nil {
		t.Fatalf("Coding Agent Send failed: %v", err)
	}
	if !strings.Contains(resp, "```bash") || !strings.Contains(resp, "cat << 'EOF' > primes.py") {
		t.Errorf("Coding Agent response invalid. Expected bash markdown block. Got:\n%s", resp)
	}

	// 4. Default
	resp, err = agent.Send(ctx, "Tell me a joke")
	if err != nil {
		t.Fatalf("Default Send failed: %v", err)
	}
	if !strings.Contains(resp, "Mock agent response") {
		t.Errorf("Default response invalid. Expected generic mock message. Got:\n%s", resp)
	}
}
