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

func TestMockAgent_TicketGeneration_StrictTrigger(t *testing.T) {
	agent := NewMockAgent()

	// Case 1: Coding Prompt containing app_spec.txt (Context) -> Should be Default (Bash No-Op) or Code, NOT JSON
	prompt := "Please implement this feature. Context: app_spec.txt content..."
	resp, _ := agent.Send(context.Background(), prompt)
	if strings.HasPrefix(strings.TrimSpace(resp), "[") {
		t.Errorf("Coding prompt with app_spec.txt triggered JSON response! Should be default bash.\nResponse: %s", resp)
	}
	if !strings.Contains(resp, "```bash") {
		t.Errorf("Coding prompt should return bash block. Got: %s", resp)
	}

	// Case 2: TPM Prompt -> Should be JSON
	promptTPM := "You are a Technical Program Manager. Analyze app_spec.txt..."
	respTPM, _ := agent.Send(context.Background(), promptTPM)
	if !strings.HasPrefix(strings.TrimSpace(respTPM), "[") {
		t.Errorf("TPM prompt failed to trigger JSON response.\nResponse: %s", respTPM)
	}
}

func TestMockAgent_PrimesImplementation_Priority(t *testing.T) {
	agent := NewMockAgent()

	// Prompt containing BOTH primes.py (Goal) AND app_spec.txt (Context)
	// Should return Implementation (Bash), NOT JSON.
	prompt := "Implement primes.py based on app_spec.txt"
	resp, _ := agent.Send(context.Background(), prompt)

	if !strings.Contains(resp, "cat << 'EOF' > primes.py") {
		t.Errorf("Primes prompt with spec context failed to trigger implementation response.\nResponse: %s", resp)
	}
	if strings.HasPrefix(strings.TrimSpace(resp), "[") {
		t.Errorf("Primes prompt triggered JSON response! Priority check failed.")
	}
}
