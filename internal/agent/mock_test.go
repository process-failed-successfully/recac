package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent_ManagerHeuristic(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	// Case 1: All features passed -> Sign Off
	promptPassed := "PROJECT MANAGER\nAll features passed."
	resp, err := agent.Send(ctx, promptPassed)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(resp, "PROJECT_SIGNED_OFF") {
		t.Errorf("Expected sign-off for passed prompt, got: %s", resp)
	}

	// Case 2: Features pending -> No Sign Off
	promptPending := "PROJECT MANAGER\nFeatures:\n- ID: 1, Status: pending"
	resp, err = agent.Send(ctx, promptPending)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if strings.Contains(resp, "PROJECT_SIGNED_OFF") {
		t.Errorf("Expected NO sign-off for pending prompt, got: %s", resp)
	}
	if !strings.Contains(resp, "Please complete the pending/failed features") {
		t.Errorf("Expected instruction to complete features, got: %s", resp)
	}

	// Case 3: Incomplete -> No Sign Off
	promptIncomplete := "PROJECT MANAGER\nFeatures incomplete."
	resp, err = agent.Send(ctx, promptIncomplete)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if strings.Contains(resp, "PROJECT_SIGNED_OFF") {
		t.Errorf("Expected NO sign-off for incomplete prompt, got: %s", resp)
	}

	// Case 4: Failed Features (Smoke Test Scenario) -> No Sign Off
	promptFailed := "PROJECT MANAGER\nQA Report: 0/2 features passing (0.0%)\nFailed Features:\n- [functional] Must correctly identify prime numbers"
	resp, err = agent.Send(ctx, promptFailed)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if strings.Contains(resp, "PROJECT_SIGNED_OFF") {
		t.Errorf("Expected NO sign-off for failed features prompt, got: %s", resp)
	}
	if !strings.Contains(resp, "Please complete the pending/failed features") {
		t.Errorf("Expected instruction to complete failed features, got: %s", resp)
	}
}

func TestMockAgent_CodingHeuristic(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	// Case 1: Primes keyword -> Generate Code
	promptPrimes := "Please write a script to calculate primes."
	resp, err := agent.Send(ctx, promptPrimes)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(resp, "cat <<EOF > primes.py") {
		t.Errorf("Expected primes script generation, got: %s", resp)
	}

	// Case 2: Tag [PRIMES] -> Generate Code
	promptTag := "Task [PRIMES] description."
	resp, err = agent.Send(ctx, promptTag)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(resp, "cat <<EOF > primes.py") {
		t.Errorf("Expected primes script generation, got: %s", resp)
	}
}

func TestMockAgent_TPMHeuristic(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	// Should match
	prompt := "You are the Technical Program Manager. Please generate tickets."
	response, err := agent.Send(ctx, prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(response, "\"type\": \"Story\"") {
		t.Error("TPM heuristic failed to match valid prompt")
	}

	// Should NOT match (False positive check)
	promptFalse := "I am the Coding Agent working on tickets."
	respFalse, _ := agent.Send(ctx, promptFalse)
	if strings.Contains(respFalse, "\"type\": \"Story\"") {
		t.Error("TPM heuristic falsely matched coding agent prompt")
	}
}

func TestMockAgent_Initializer(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	prompt := "YOUR ROLE: INITIALIZER AGENT"
	resp, _ := agent.Send(ctx, prompt)
	if !strings.Contains(resp, "agent-bridge import") {
		t.Error("Initializer heuristic failed")
	}
}

func TestMockAgent_DefaultResponse(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	prompt := "This is a random prompt that should trigger fallback"
	response, err := agent.Send(ctx, prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(response, "Mock agent response") {
		t.Errorf("Response missing prefix, got: %s", response)
	}
	if !strings.Contains(response, "```bash") {
		t.Error("Fallback response missing bash block (circuit breaker safety)")
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
