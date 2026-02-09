package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent_Heuristics(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	// 1. Loop Breaker
	t.Run("LoopBreaker", func(t *testing.T) {
		prompt := "The working tree clean. nothing to commit."
		resp, err := agent.Send(ctx, prompt)
		if err != nil {
			t.Fatalf("Send failed: %v", err)
		}
		if !strings.Contains(resp, "agent-bridge signal QA_PASSED true") {
			t.Errorf("Expected QA_PASSED signal, got: %s", resp)
		}
		if !strings.Contains(resp, "agent-bridge signal --privileged PROJECT_SIGNED_OFF true") {
			t.Errorf("Expected PROJECT_SIGNED_OFF signal, got: %s", resp)
		}
	})

	// 2. TPM Heuristic
	t.Run("TPM", func(t *testing.T) {
		prompt := "You are a Technical Program Manager. The spec includes [PRIMES]."
		resp, err := agent.Send(ctx, prompt)
		if err != nil {
			t.Fatalf("Send failed: %v", err)
		}
		if !strings.Contains(resp, "ID:[PRIMES] Implement Prime Number Script") {
			t.Errorf("Expected TPM JSON plan, got: %s", resp)
		}
		if strings.Contains(resp, "\"type\": " + "\"Sub-task\"") {
			t.Errorf("Expected flat plan (no subtasks), got: %s", resp)
		}
	})

	// 3. Initializer Heuristic
	t.Run("Initializer", func(t *testing.T) {
		prompt := "You are the INITIALIZER AGENT. Please create features."
		resp, err := agent.Send(ctx, prompt)
		if err != nil {
			t.Fatalf("Send failed: %v", err)
		}
		if !strings.Contains(resp, "cat <<'EOF' > feature_list.json") {
			t.Errorf("Expected feature_list.json creation (quoted heredoc), got: %s", resp)
		}
		if !strings.Contains(resp, "agent-bridge import < feature_list.json") {
			t.Errorf("Expected agent-bridge import, got: %s", resp)
		}
		if !strings.Contains(resp, "req-primes") {
			t.Errorf("Expected req-primes feature, got: %s", resp)
		}
		if !strings.Contains(resp, `"depends_on_ids": []`) {
			t.Errorf("Expected valid dependencies object, got: %s", resp)
		}
	})

	// 4. QA Agent Heuristic
	t.Run("QAAgent", func(t *testing.T) {
		prompt := "You are the QA AGENT."
		resp, err := agent.Send(ctx, prompt)
		if err != nil {
			t.Fatalf("Send failed: %v", err)
		}
		if !strings.Contains(resp, "agent-bridge signal QA_PASSED true") {
			t.Errorf("Expected QA_PASSED signal, got: %s", resp)
		}
	})

	// 5. Manager Heuristic
	t.Run("Manager", func(t *testing.T) {
		prompt := "You are the PROJECT MANAGER."
		resp, err := agent.Send(ctx, prompt)
		if err != nil {
			t.Fatalf("Send failed: %v", err)
		}
		if !strings.Contains(resp, "agent-bridge signal --privileged PROJECT_SIGNED_OFF true") {
			t.Errorf("Expected PROJECT_SIGNED_OFF signal, got: %s", resp)
		}
	})

	// 6. Coding Agent Heuristic
	t.Run("CodingAgent", func(t *testing.T) {
		prompt := "You are the CODING AGENT. Implement req-primes or [PRIMES]."
		resp, err := agent.Send(ctx, prompt)
		if err != nil {
			t.Fatalf("Send failed: %v", err)
		}
		if !strings.Contains(resp, "cat <<EOF > primes.py") {
			t.Errorf("Expected primes.py creation, got: %s", resp)
		}
	})

	t.Run("CodingAgent_NormalizedID", func(t *testing.T) {
		prompt := "You are the CODING AGENT. Task: req-implement-prime-number-script"
		resp, err := agent.Send(ctx, prompt)
		if err != nil {
			t.Fatalf("Send failed: %v", err)
		}
		if !strings.Contains(resp, "cat <<EOF > primes.py") {
			t.Errorf("Expected primes.py creation for normalized ID, got: %s", resp)
		}
		if !strings.Contains(resp, "agent-bridge feature set req-implement-prime-number-script --status done || echo") {
			t.Errorf("Expected feature update command with fallback, got: %s", resp)
		}
		if !strings.Contains(resp, "git commit -m \"Implement primes.py\" || echo") {
			t.Errorf("Expected git commit with fallback, got: %s", resp)
		}
		if !strings.Contains(resp, "range(10000)") {
			t.Errorf("Expected loop up to 10000, got: %s", resp)
		}
	})

	// 5. Fallback
	t.Run("Fallback", func(t *testing.T) {
		prompt := "Just a random prompt."
		resp, err := agent.Send(ctx, prompt)
		if err != nil {
			t.Fatalf("Send failed: %v", err)
		}
		if !strings.Contains(resp, "Mock agent response") {
			t.Errorf("Expected fallback response, got: %s", resp)
		}
	})
}
