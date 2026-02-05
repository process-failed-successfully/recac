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
	if !strings.Contains(resp, "Please complete the pending features") {
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
