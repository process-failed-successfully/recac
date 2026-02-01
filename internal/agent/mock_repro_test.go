package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent_PrimePython_Repro(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	// Simulate the prompt sent by 'recac jira generate-from-spec' using the TPM agent template
	// We don't need the exact full template, just enough to trigger the match
	prompt := `You are an expert Technical Program Manager...
### Application Specification:
### ID:[PRIMES] Prime Number Script
CRITICAL INSTRUCTION: You MUST create exactly ONE ticket...`

	resp, err := agent.Send(ctx, prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Verify JSON keys match ticketNode struct (Title, Description)
	if !strings.Contains(resp, `"title": "ID:[PRIMES]`) {
		t.Errorf("Expected JSON response with 'title' key, got: %s", resp)
	}
	if !strings.Contains(resp, `"description":`) {
		t.Errorf("Expected JSON response with 'description' key, got: %s", resp)
	}
	// Verify Repo URL is injected (to pass validation in CLI)
	if !strings.Contains(resp, `Repo: https://github.com/process-failed-successfully/recac-jira-e2e`) {
		t.Errorf("Expected JSON response to contain Repo URL, got: %s", resp)
	}
}
