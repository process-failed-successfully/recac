package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent_Initializer(t *testing.T) {
	agent := NewMockAgent()
	prompt := "Please run agent-bridge import to initialize."
	resp, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(resp, "cat << 'EOF' > feature_list.json") {
		t.Errorf("Expected feature list creation, got: %s", resp)
	}
	if !strings.Contains(resp, "agent-bridge import") {
		t.Errorf("Expected agent-bridge import command, got: %s", resp)
	}
}

func TestMockAgent_NoOp(t *testing.T) {
	agent := NewMockAgent()
	prompt := "Just a random chat."
	resp, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(resp, "# no-op") {
		t.Errorf("Expected # no-op, got: %s", resp)
	}
}

func TestMockAgent_Completion(t *testing.T) {
	agent := NewMockAgent()

	// Test case 1: "nothing to commit"
	prompt1 := "git commit output: nothing to commit, working tree clean"
	resp1, err := agent.Send(context.Background(), prompt1)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(resp1, "agent-bridge signal COMPLETED true") {
		t.Errorf("Expected completion signal for 'nothing to commit', got: %s", resp1)
	}

	// Test case 2: "Nothing to commit" (capitalized, from echo fallback)
	prompt2 := "command output: Nothing to commit"
	resp2, err := agent.Send(context.Background(), prompt2)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(resp2, "agent-bridge signal COMPLETED true") {
		t.Errorf("Expected completion signal for 'Nothing to commit', got: %s", resp2)
	}
}

func TestMockAgent_CompletionPriority(t *testing.T) {
	agent := NewMockAgent()
	// Prompt containing both "primes.py" (context) and "nothing to commit" (output)
	prompt := "history: wrote primes.py... output: nothing to commit, working tree clean"
	resp, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Should prioritize completion over implementation
	if strings.Contains(resp, "cat << 'EOF' > primes.py") {
		t.Errorf("MockAgent stuck in loop: returned implementation again instead of completion signal")
	}
	if !strings.Contains(resp, "agent-bridge signal COMPLETED true") {
		t.Errorf("Expected completion signal, got: %s", resp)
	}
}

func TestMockAgent_QA(t *testing.T) {
	agent := NewMockAgent()
	prompt := "Context: ... YOUR ROLE - QA AGENT ..."
	resp, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(resp, "agent-bridge signal QA_PASSED true") {
		t.Errorf("Expected QA_PASSED signal, got: %s", resp)
	}
}

func TestMockAgent_Manager(t *testing.T) {
	agent := NewMockAgent()
	prompt := "Context: ... YOUR ROLE - PROJECT MANAGER ..."
	resp, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(resp, "agent-bridge signal PROJECT_SIGNED_OFF true") {
		t.Errorf("Expected PROJECT_SIGNED_OFF signal, got: %s", resp)
	}
}

func TestMockAgent_ManagerPriority(t *testing.T) {
	agent := NewMockAgent()
	// Prompt containing both "primes.py" (task context) and "YOUR ROLE - PROJECT MANAGER"
	prompt := "History: implemented primes.py ... YOUR ROLE - PROJECT MANAGER ... QA Report for primes.py"
	resp, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Should prioritize Manager Sign-off over Implementation
	if strings.Contains(resp, "cat << 'EOF' > primes.py") {
		t.Errorf("MockAgent stuck in loop: returned implementation script instead of Manager sign-off")
	}
	if !strings.Contains(resp, "agent-bridge signal PROJECT_SIGNED_OFF true") {
		t.Errorf("Expected PROJECT_SIGNED_OFF signal, got: %s", resp)
	}
}
