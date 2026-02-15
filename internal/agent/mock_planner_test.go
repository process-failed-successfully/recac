package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent_Planner_JSON(t *testing.T) {
	agent := NewMockAgent()
	// Simulate the planner prompt
	prompt := "## ROLE: Lead Software Architect\n\nYou are a Lead Software Architect.\nGiven the following application specification..."

	resp, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(resp, "\"project_name\": \"Mock Project\"") {
		t.Errorf("Expected JSON response for planner prompt, got:\n%s", resp)
	}
}
