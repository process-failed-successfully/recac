package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent_Primes(t *testing.T) {
	agent := NewMockAgent()

	// 1. Test Ticket Generation
	prompt := "CRITICAL INSTRUCTION: You MUST create exactly ONE ticket. Type: Task. ID: [PRIMES]"
	resp, _ := agent.Send(context.Background(), prompt)
	if !strings.Contains(resp, `"id": "PRIMES"`) {
		t.Errorf("Ticket generation failed: %s", resp)
	}

	// 2. Test Initialization
	prompt = "Please initialize the project. agent-bridge import feature_list.json"
	resp, _ = agent.Send(context.Background(), prompt)
	if !strings.Contains(resp, "cat <<EOF > feature_list.json") {
		t.Errorf("Initialization failed: %s", resp)
	}

	// 3. Test Implementation
	prompt = "Task: [PRIMES] Create a python script named 'primes.py'"
	resp, _ = agent.Send(context.Background(), prompt)
	if !strings.Contains(resp, "cat <<EOF > primes.py") {
		t.Errorf("Implementation failed (file creation): %s", resp)
	}
	if !strings.Contains(resp, "python3 primes.py") {
		t.Errorf("Implementation failed (execution): %s", resp)
	}
}
