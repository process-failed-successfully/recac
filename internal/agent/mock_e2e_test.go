package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent_E2E_Heuristics(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	t.Run("Planning Phase (TPM)", func(t *testing.T) {
		prompt := "You are a Technical Program Manager"
		resp, err := agent.Send(ctx, prompt)
		if err != nil {
			t.Fatalf("Send failed: %v", err)
		}
		// TPM returns []ticketNode JSON for recac CLI
		if !strings.Contains(resp, `ID:[PRIMES]`) {
			t.Errorf("Expected ticketNode JSON with ID:[PRIMES] for prompt '%s', got: %s", prompt, resp)
		}
	})

	t.Run("Planning Phase (Planner)", func(t *testing.T) {
		prompt := "Please break down this task"
		resp, err := agent.Send(ctx, prompt)
		if err != nil {
			t.Fatalf("Send failed: %v", err)
		}
		// Planner returns feature_list.json bash script
		if !strings.Contains(resp, `"id": "req-primes"`) {
			t.Errorf("Expected planning JSON for prompt '%s', got: %s", prompt, resp)
		}
	})

	t.Run("Coding Phase", func(t *testing.T) {
		prompts := []string{
			"Implement primes.py",
			"Write a python script to implement primes",
		}
		for _, p := range prompts {
			resp, err := agent.Send(ctx, p)
			if err != nil {
				t.Fatalf("Send failed: %v", err)
			}
			if !strings.Contains(resp, "cat << 'EOF' > primes.py") {
				t.Errorf("Expected bash script for prompt '%s', got: %s", p, resp)
			}
			if !strings.Contains(resp, "python3 primes.py") {
				t.Errorf("Expected execution command for prompt '%s', got: %s", p, resp)
			}
		}
	})

	t.Run("QA Phase", func(t *testing.T) {
		resp, err := agent.Send(ctx, "You are a QA Agent")
		if err != nil {
			t.Fatalf("Send failed: %v", err)
		}
		if resp != "QA_PASSED" {
			t.Errorf("Expected 'QA_PASSED', got: %s", resp)
		}
	})

	t.Run("Manager Phase", func(t *testing.T) {
		resp, err := agent.Send(ctx, "You are a Manager Agent")
		if err != nil {
			t.Fatalf("Send failed: %v", err)
		}
		if resp != "PROJECT_SIGNED_OFF" {
			t.Errorf("Expected 'PROJECT_SIGNED_OFF', got: %s", resp)
		}
	})
}
