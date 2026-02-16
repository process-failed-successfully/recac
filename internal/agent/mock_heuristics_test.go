package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent_Heuristics(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	// 1. Planning Phase (Ticket Generation)
	t.Run("Planning_TicketGeneration", func(t *testing.T) {
		prompt := "This prompt contains CRITICAL INSTRUCTION FOR TICKET GENERATION..."
		resp, err := agent.Send(ctx, prompt)
		if err != nil {
			t.Fatalf("Send failed: %v", err)
		}
		if !strings.Contains(resp, "\"tickets\": [") {
			t.Errorf("Expected JSON ticket response, got: %s", resp)
		}
	})

	// 2. Planning Phase (Project Manager Role)
	t.Run("Planning_ProjectManager", func(t *testing.T) {
		prompt := "You are in role - project manager"
		resp, err := agent.Send(ctx, prompt)
		if err != nil {
			t.Fatalf("Send failed: %v", err)
		}
		if !strings.Contains(resp, "\"tickets\": [") {
			t.Errorf("Expected JSON ticket response, got: %s", resp)
		}
	})

	// 3. Coding Phase (Primes Script)
	t.Run("Coding_Primes", func(t *testing.T) {
		prompt := "Implement the script id:[primes]"
		resp, err := agent.Send(ctx, prompt)
		if err != nil {
			t.Fatalf("Send failed: %v", err)
		}
		if strings.Contains(resp, "\"tickets\": [") {
			t.Errorf("Expected Bash script, got JSON tickets (Planning heuristic triggered incorrectly)")
		}
		if !strings.Contains(resp, "cat <<EOF > primes.py") {
			t.Errorf("Expected Bash script creation, got: %s", resp)
		}
		if !strings.Contains(resp, "range(10000)") {
			t.Errorf("Expected range(10000) in python script, got: %s", resp)
		}
	})

	// 4. Default
	t.Run("Default", func(t *testing.T) {
		prompt := "Just a normal prompt"
		resp, err := agent.Send(ctx, prompt)
		if err != nil {
			t.Fatalf("Send failed: %v", err)
		}
		if !strings.Contains(resp, "Mock agent response") {
			t.Errorf("Expected default mock response, got: %s", resp)
		}
	})
}
