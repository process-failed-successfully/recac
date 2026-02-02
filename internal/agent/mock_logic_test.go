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

	// Test case 3: Conflict priority - "primes.py" AND "nothing to commit"
	// Should prioritize completion over re-implementation
	prompt3 := "I implemented primes.py. Output: nothing to commit, working tree clean."
	resp3, err := agent.Send(context.Background(), prompt3)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if strings.Contains(resp3, "cat << 'EOF' > primes.py") {
		t.Errorf("Agent tried to re-implement primes.py instead of completing!")
	}
	if !strings.Contains(resp3, "agent-bridge signal COMPLETED true") {
		t.Errorf("Expected completion signal when both keywords present, got: %s", resp3)
	}
}

func TestMockAgent_Roles(t *testing.T) {
	agent := NewMockAgent()

	// Test QA Role
	promptQA := "YOUR ROLE - QA AGENT. Please verify."
	respQA, err := agent.Send(context.Background(), promptQA)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(respQA, "agent-bridge signal QA_PASSED true") {
		t.Errorf("Expected QA_PASSED signal for QA role, got: %s", respQA)
	}

	// Test Manager Role
	promptMgr := "YOUR ROLE - PROJECT MANAGER. Please review."
	respMgr, err := agent.Send(context.Background(), promptMgr)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(respMgr, "agent-bridge signal PROJECT_SIGNED_OFF true") {
		t.Errorf("Expected PROJECT_SIGNED_OFF signal for Manager role, got: %s", respMgr)
	}
}
