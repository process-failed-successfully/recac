package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent_PrimesResponse(t *testing.T) {
	agent := NewMockAgent()
	resp, err := agent.Send(context.Background(), "Implement primes.py")
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if strings.Contains(resp, "'''bash") {
		t.Error("Mock agent response contains invalid markdown code block delimiter '''bash")
	}

	if !strings.Contains(resp, "```bash") {
		t.Error("Mock agent response missing valid markdown code block delimiter ```bash")
	}
}

func TestMockAgent_PrimesResponse_WithIDs(t *testing.T) {
	agent := NewMockAgent()
	// Test with only feature IDs, which might happen if descriptions are missing/formatted differently
	prompt := "Implement feature req-primes-py-exists and req-primes-json-exists"
	resp, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(resp, "cat << 'EOF' > primes.py") {
		t.Error("Mock agent did not return primes implementation when prompted with feature IDs")
	}
}
