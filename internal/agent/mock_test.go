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
