package agent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestMockAgent_TPM_Response(t *testing.T) {
	agent := NewMockAgent()
	prompt := "You are an expert Technical Program Manager (TPM)..."

	resp, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// It should return a valid JSON
	var result interface{}
	if err := json.Unmarshal([]byte(resp), &result); err != nil {
		t.Errorf("expected JSON response, but got: %s", resp)
	}

	// Verify it contains expected fields (optional but good)
	if !strings.Contains(resp, `"title"`) {
		t.Errorf("response should contain ticket fields, got: %s", resp)
	}
}
