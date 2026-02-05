package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent_PrimeHeuristic(t *testing.T) {
	agent := NewMockAgent()
	prompt := "Please implement a python script named 'primes.py' that calculates prime numbers."

	resp, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("agent.Send failed: %v", err)
	}

	expectedStrings := []string{
		"cat << 'EOF' > primes.py",
		"def is_prime(n):",
		"python3 primes.py",
		"git commit -m \"Implement prime number calculator\"",
	}

	for _, s := range expectedStrings {
		if !strings.Contains(resp, s) {
			t.Errorf("Response missing expected string: %q\nResponse:\n%s", s, resp)
		}
	}
}
