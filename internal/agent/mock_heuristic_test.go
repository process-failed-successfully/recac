package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent_Heuristic_CodingAgent_vs_TPM(t *testing.T) {
	agent := NewMockAgent()

	// Simulate a prompt from Coding Agent template
	// It contains "app_spec.txt" which triggers the TPM heuristic in the buggy version
	prompt := `## YOUR ROLE - CODING AGENT
    ...
    cat app_spec.txt
    ...
    `

	resp, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Agent failed: %v", err)
	}

	// We expect the fallback response (Bash block), NOT the TPM response (JSON)
	if strings.HasPrefix(strings.TrimSpace(resp), "[") {
		t.Errorf("MockAgent returned JSON (TPM response) for Coding Agent prompt. Response:\n%s", resp)
	}

	if !strings.Contains(resp, "```bash") {
		t.Errorf("MockAgent did not return bash block (Fallback response) for Coding Agent prompt. Response:\n%s", resp)
	}
}
