package agent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestMockAgent_TPMHeuristic(t *testing.T) {
	agent := NewMockAgent()
	prompt := "You are an expert Technical Program Manager (TPM)... read app_spec.txt... generate tickets..."

	resp, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Verify JSON structure
	var tickets []interface{}
	if err := json.Unmarshal([]byte(resp), &tickets); err != nil {
		t.Errorf("Response is not valid JSON: %v\nResponse: %s", err, resp)
	}

	if len(tickets) == 0 {
		t.Error("Expected at least one ticket")
	}
}

func TestMockAgent_DefaultFallback(t *testing.T) {
	agent := NewMockAgent()
	prompt := "Hello world"

	resp, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(resp, "I received your prompt") {
		t.Errorf("Expected generic response, got: %s", resp)
	}
}
