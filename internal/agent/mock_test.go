package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent(t *testing.T) {
	agent := NewMockAgent()

	prompt := "This is a test prompt that is long enough to be truncated"
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

func TestTruncateString(t *testing.T) {
	s := "hello world"
	if truncateString(s, 5) != "hello" {
		t.Errorf("Expected 'hello', got '%s'", truncateString(s, 5))
	}
	if truncateString(s, 20) != "hello world" {
		t.Errorf("Expected 'hello world', got '%s'", truncateString(s, 20))
	}
}

func TestMockAgent_PrimesScenario(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	// 1. Test TPM Prompt
	tpmPrompt := "You are an expert Technical Program Manager... [PRIMES] ..."
	resp, err := agent.Send(ctx, tpmPrompt)
	if err != nil {
		t.Fatalf("Failed to send TPM prompt: %v", err)
	}
	if !strings.Contains(resp, `"type": "Task"`) {
		t.Errorf("Expected JSON plan for TPM prompt, got: %s", resp)
	}
	if strings.Contains(resp, "cat << 'EOF' > primes.py") {
		t.Error("TPM response should not contain implementation script")
	}

	// 2. Test Coding Prompt
	codingPrompt := "## YOUR ROLE - CODING AGENT ... [PRIMES] ..."
	resp, err = agent.Send(ctx, codingPrompt)
	if err != nil {
		t.Fatalf("Failed to send Coding prompt: %v", err)
	}
	if !strings.Contains(resp, "cat << 'EOF' > primes.py") {
		t.Errorf("Expected implementation script for Coding prompt, got: %s", resp)
	}
	if strings.Contains(resp, `"type": "Task"`) {
		t.Error("Coding response should not contain JSON plan")
	}

	// 3. Test Ambiguous Prompt (Fallback)
	ambiguousPrompt := "Just do the [PRIMES] thing"
	resp, err = agent.Send(ctx, ambiguousPrompt)
	if err != nil {
		t.Fatalf("Failed to send Ambiguous prompt: %v", err)
	}
	if !strings.Contains(resp, "cat << 'EOF' > primes.py") {
		t.Errorf("Expected fallback to implementation, got: %s", resp)
	}

	// 4. Test Completion (History detection)
	// Simulate a prompt that includes the history of the previous action
	historyPrompt := "## YOUR ROLE - CODING AGENT ... [PRIMES] ... RECENT HISTORY: ... cat << 'EOF' > primes.py ..."
	resp, err = agent.Send(ctx, historyPrompt)
	if err != nil {
		t.Fatalf("Failed to send History prompt: %v", err)
	}
	if strings.Contains(resp, "cat << 'EOF' > primes.py") {
		t.Error("Should NOT repeat implementation script if already in history")
	}
	if !strings.Contains(resp, "agent-bridge signal COMPLETED true") {
		t.Errorf("Expected completion signal, got: %s", resp)
	}
}

func TestMockAgent_PrimesScenario_RealWorld(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	// These prompts mimic the actual prompts sent by the E2E runner for each feature,
	// which lack the [PRIMES] tag.
	prompts := []string{
		"Feature ID: req-script-calculates-primes-corre\nDescription: Script calculates primes correctly",
		"Feature ID: req-output-is-saved-to-primes-json\nDescription: Output is saved to primes.json",
		"Feature ID: req-contains-exactly-1229-primes\nDescription: Contains exactly 1229 primes",
	}

	for _, p := range prompts {
		resp, err := agent.Send(ctx, p)
		if err != nil {
			t.Fatalf("Failed to send prompt %q: %v", p, err)
		}
		// Expect the implementation script
		if !strings.Contains(resp, "cat << 'EOF' > primes.py") {
			t.Errorf("Heuristic failed for prompt %q. Got fallback response.", p)
		}
	}
}
