package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent_Heuristic_Manager(t *testing.T) {
	agent := NewMockAgent()

	// Simulate a prompt from Manager Review template
	// It contains "PROJECT MANAGER" header
	prompt := `## YOUR ROLE - PROJECT MANAGER
    ...
    qa_report
    ...
    `

	resp, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Agent failed: %v", err)
	}

	// We expect the Manager response (Approval/Sign-off), NOT the Fallback (Bash block)
	if !strings.Contains(resp, "PROJECT_SIGNED_OFF") && !strings.Contains(resp, "manager") {
		t.Errorf("MockAgent did not return Manager response for Manager prompt. Response:\n%s", resp)
	}

	if strings.Contains(resp, "Mock Agent is alive") {
		t.Errorf("MockAgent returned Fallback response for Manager prompt. Response:\n%s", resp)
	}
}
