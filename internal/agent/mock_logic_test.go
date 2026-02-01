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
	// Simulate prompt containing git status output indicating nothing to commit
	prompt := "Command Output:\nOn branch master\nnothing to commit, working tree clean\n"
	resp, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(resp, "agent-bridge feature set req-primes-py-exists") {
		t.Errorf("Expected agent-bridge feature set for req-primes-py-exists, got: %s", resp)
	}
	if !strings.Contains(resp, "agent-bridge feature set req-primes-json-contains-correct-primes") {
		t.Errorf("Expected agent-bridge feature set for req-primes-json-contains-correct-primes, got: %s", resp)
	}
	if !strings.Contains(resp, "--status done") {
		t.Errorf("Expected status done, got: %s", resp)
	}
}
