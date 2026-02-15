package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent_InjectedFeatures(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	// Simulate environment variable set by smoke test
	t.Setenv("RECAC_INJECTED_FEATURES", `{"project_name":"ID:[PRIMES-1] Create primes.py"}`)

	// Send a generic prompt that lacks context
	prompt := "Please complete the task."
	resp, err := agent.Send(ctx, prompt)

	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Expect the Primes script because the environment has the context
	expected := "def is_prime(n):"
	if !strings.Contains(resp, expected) {
		t.Errorf("Expected output to contain %q, got:\n%s", expected, resp)
	}

	// Also verify that the default response is NOT returned
	notExpected := "Mock agent response"
	if strings.Contains(resp, notExpected) {
		t.Errorf("Expected output NOT to contain %q, but it did:\n%s", notExpected, resp)
	}
}
