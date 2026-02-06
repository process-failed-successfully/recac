package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent_Heuristics(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	t.Run("TPM Role - JSON Plan", func(t *testing.T) {
		prompt := "You are an expert Technical Program Manager (TPM). CRITICAL INSTRUCTION FOR TICKET GENERATION: Create a SINGLE Ticket."
		resp, err := agent.Send(ctx, prompt)
		if err != nil {
			t.Fatalf("Send failed: %v", err)
		}
		if !strings.Contains(resp, `"id": "PRIMES"`) {
			t.Errorf("Expected JSON plan with 'PRIMES' ID, got: %s", resp)
		}
		if !strings.Contains(resp, `"type": "Task"`) {
			t.Errorf("Expected JSON plan with 'Task' type, got: %s", resp)
		}
	})

	t.Run("Developer Role - Bash Script", func(t *testing.T) {
		prompt := "Implement a python script named 'primes.py'. [PRIMES]"
		resp, err := agent.Send(ctx, prompt)
		if err != nil {
			t.Fatalf("Send failed: %v", err)
		}
		if !strings.Contains(resp, "#!/bin/bash") {
			t.Errorf("Expected bash script, got: %s", resp)
		}
		if !strings.Contains(resp, "cat << 'EOF' > primes.py") {
			t.Errorf("Expected file creation, got: %s", resp)
		}
		if !strings.Contains(resp, "git config --global") {
			t.Errorf("Expected git config, got: %s", resp)
		}
	})

	t.Run("Fallback", func(t *testing.T) {
		prompt := "Just a random chat message."
		resp, err := agent.Send(ctx, prompt)
		if err != nil {
			t.Fatalf("Send failed: %v", err)
		}
		if !strings.Contains(resp, "Mock agent response") {
			t.Errorf("Expected fallback response, got: %s", resp)
		}
		if strings.Contains(resp, "#!/bin/bash") {
			t.Errorf("Fallback should not return bash script")
		}
	})
}
