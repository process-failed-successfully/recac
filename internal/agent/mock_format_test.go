package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent_Coding_Response_Format(t *testing.T) {
	agent := NewMockAgent()
	prompt := "CODING AGENT"

	resp, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(resp, "```bash") && !strings.Contains(resp, "```sh") {
		t.Errorf("response should contain markdown code block, got: %s", resp)
	}
}
