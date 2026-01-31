package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent_DefaultResponse_HasNoOp(t *testing.T) {
	agent := NewMockAgent()
	resp, err := agent.Send(context.Background(), "Some random prompt")
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(resp, "```bash") {
		t.Errorf("Response missing bash block: %s", resp)
	}
	if !strings.Contains(resp, "# no-op") {
		t.Errorf("Response missing no-op comment: %s", resp)
	}
}
