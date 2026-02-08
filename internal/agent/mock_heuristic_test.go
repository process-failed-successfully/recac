package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent_Heuristics(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	t.Run("Initializer", func(t *testing.T) {
		prompt := "YOUR ROLE - INITIALIZER AGENT. Please analyze [PRIMES]"
		resp, err := agent.Send(ctx, prompt)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(resp, "req-primes") {
			t.Errorf("expected feature list, got: %s", resp)
		}
	})

	t.Run("TPM_Primes", func(t *testing.T) {
		prompt := "You are an expert Technical Program Manager (TPM). Analyze this spec: [PRIMES]"
		resp, err := agent.Send(ctx, prompt)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Should return JSON list of tickets, not bash script
		if strings.Contains(resp, "cat << 'EOF'") {
			t.Errorf("expected JSON tickets, got implementation script: %s", resp)
		}
		if !strings.Contains(resp, `"id": "PRIMES"`) {
			t.Errorf("expected ticket with ID PRIMES, got: %s", resp)
		}
	})

	t.Run("Coder_Primes", func(t *testing.T) {
		prompt := "YOUR ROLE - CODING AGENT. Implement [PRIMES] primes.py"
		resp, err := agent.Send(ctx, prompt)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(resp, "cat << 'EOF' > primes.py") {
			t.Errorf("expected bash script to create primes.py, got: %s", resp)
		}
		if !strings.Contains(resp, "python3 primes.py") {
			t.Errorf("expected execution command, got: %s", resp)
		}
	})

	t.Run("Default", func(t *testing.T) {
		prompt := "Hello there"
		resp, err := agent.Send(ctx, prompt)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(resp, "I received your prompt") {
			t.Errorf("expected default response, got: %s", resp)
		}
	})
}
