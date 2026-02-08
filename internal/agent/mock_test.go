package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent(t *testing.T) {
	agent := NewMockAgent()

	// Use a prompt that doesn't trigger any heuristics
	prompt := "This is a generic test prompt that should just echo"
	response, err := agent.Send(context.Background(), prompt)

	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(response, "Mock agent response") {
		t.Errorf("Response missing prefix, got: %s", response)
	}

	if !strings.Contains(response, "I received your prompt") {
		t.Errorf("Response missing body, got: %s", response)
	}
}

func TestMockAgent_SmokeTestLogic(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	// 1. Initializer (feature_list.json)
	t.Run("Initializer", func(t *testing.T) {
		prompt := "You are the Technical Program Manager. Please create feature_list.json from the spec: [PRIMES]"
		resp, err := agent.Send(ctx, prompt)
		if err != nil {
			t.Fatalf("Error: %v", err)
		}
		if !strings.Contains(resp, "cat <<EOF > feature_list.json") {
			t.Errorf("Expected Initializer response (feature_list.json), got: %s", resp)
		}
		if !strings.Contains(resp, "\"id\": \"PRIMES\"") {
			t.Errorf("Expected PRIMES feature in JSON, got: %s", resp)
		}
	})

	// 2. Coding Agent (Implementation)
	t.Run("Coding Agent Implementation", func(t *testing.T) {
		prompt := "You are the CODING AGENT. Implement task: [PRIMES]. create primes.py"
		resp, err := agent.Send(ctx, prompt)
		if err != nil {
			t.Fatalf("Error: %v", err)
		}
		if !strings.Contains(resp, "cat << 'EOF' > primes.py") {
			t.Errorf("Expected Coding Agent response (primes.py), got: %s", resp)
		}
		if !strings.Contains(resp, "agent-bridge feature set PRIMES --status done --passes true") {
			t.Errorf("Expected agent-bridge feature set command, got: %s", resp)
		}
	})

	// 3. Coding Agent (Completion)
	t.Run("Coding Agent Completion", func(t *testing.T) {
		prompt := "You are the CODING AGENT. All features are marked as done/passing."
		resp, err := agent.Send(ctx, prompt)
		if err != nil {
			t.Fatalf("Error: %v", err)
		}
		if !strings.Contains(resp, "agent-bridge signal COMPLETED true") {
			t.Errorf("Expected COMPLETED signal, got: %s", resp)
		}
	})

	// 4. QA Agent
	t.Run("QA Agent", func(t *testing.T) {
		prompt := "You are the QA agent. Please run verification."
		resp, err := agent.Send(ctx, prompt)
		if err != nil {
			t.Fatalf("Error: %v", err)
		}
		if !strings.Contains(resp, "agent-bridge signal QA_PASSED true") {
			t.Errorf("Expected QA_PASSED signal, got: %s", resp)
		}
	})

	// 5. Manager Agent
	t.Run("Manager Agent", func(t *testing.T) {
		prompt := "You are the Manager. Please review and sign-off."
		resp, err := agent.Send(ctx, prompt)
		if err != nil {
			t.Fatalf("Error: %v", err)
		}
		if !strings.Contains(resp, "agent-bridge signal PROJECT_SIGNED_OFF true") {
			t.Errorf("Expected PROJECT_SIGNED_OFF signal, got: %s", resp)
		}
	})

	// 6. False Positive Check (Coding Agent context in Initializer check)
	t.Run("False Positive Check", func(t *testing.T) {
		// This prompt has [PRIMES] but also "CODING AGENT". It should NOT trigger Initializer.
		// It should trigger Coding Agent logic.
		prompt := "You are the CODING AGENT. I see [PRIMES] in the spec."
		resp, err := agent.Send(ctx, prompt)
		if err != nil {
			t.Fatalf("Error: %v", err)
		}

		// Should NOT be initializer
		if strings.Contains(resp, "cat <<EOF > feature_list.json") {
			t.Errorf("Incorrectly triggered Initializer logic for Coding Agent prompt: %s", resp)
		}

		// Should trigger Coding Agent logic because of [PRIMES] and CODING AGENT
		if !strings.Contains(resp, "cat << 'EOF' > primes.py") {
			t.Errorf("Failed to trigger Coding Agent logic for prompt with [PRIMES]: %s", resp)
		}
	})
}

func TestTruncateString(t *testing.T) {
	s := "hello world"
	if truncateString(s, 5) != "hello" {
		t.Errorf("Expected 'hello', got '%s'", truncateString(s, 5))
	}
	if truncateString(s, 20) != "hello world" {
		t.Errorf("Expected 'hello world', got '%s'", truncateString(s, 20))
	}
}
