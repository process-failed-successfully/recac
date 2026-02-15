package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent_TPM_Repro(t *testing.T) {
	agent := NewMockAgent()
	prompt := "You are an expert Technical Program Manager (TPM) with deep experience in agile software development..."
	resp, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	// The CI failure is because the response starts with "Mock agent response:" instead of JSON
	if strings.HasPrefix(resp, "Mock agent response:") {
		t.Logf("Reproduced: Response is conversational, not JSON: %s", resp)
	} else {
		t.Logf("Response: %s", resp)
	}
}
