package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent(t *testing.T) {
	agent := NewMockAgent()

	prompt := "This is a test prompt that is long enough to be truncated"
	response, err := agent.Send(context.Background(), prompt)

	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(response, "Mock agent response") {
		t.Errorf("Response missing prefix, got: %s", response)
	}

	if !strings.Contains(response, "I received your prompt") {
		t.Errorf("Response missing body, got: %s", response)
	}
}

func TestMockAgent_Heuristics(t *testing.T) {
	agent := NewMockAgent()

	// 1. Test Ticket Planning
	planPrompt := "You are a Technical Program Manager. Please generate ticket..."
	planResp, _ := agent.Send(context.Background(), planPrompt)
	if !strings.Contains(planResp, "\"id\": \"[PRIMES]\"") {
		t.Errorf("Expected JSON ticket list for planning prompt, got: %s", planResp)
	}

	// 2. Test Execution (Prime Python)
	execPrompt := "Write a python script to calculate prime numbers."
	execResp, _ := agent.Send(context.Background(), execPrompt)
	if !strings.Contains(execResp, "def is_prime(n):") {
		t.Errorf("Expected python code for execution prompt, got: %s", execResp)
	}
	if !strings.Contains(execResp, "primes.json") {
		t.Errorf("Expected primes.json generation in execution prompt, got: %s", execResp)
	}

	// 3. Test Completion (Prime + Nothing to commit)
	completionPrompt := "I ran the python script to calculate prime numbers. Result: nothing to commit, working tree clean"
	compResp, _ := agent.Send(context.Background(), completionPrompt)
	if !strings.Contains(compResp, "agent-bridge feature set \"[PRIMES]\" --status done") {
		t.Errorf("Expected completion command for 'nothing to commit', got: %s", compResp)
	}
	if !strings.Contains(compResp, "agent-bridge signal PROJECT_SIGNED_OFF true --privileged") {
		t.Errorf("Expected PROJECT_SIGNED_OFF signal for completion, got: %s", compResp)
	}

	// 4. Test QA Phase
	qaPrompt := "Here is the QA report for the feature."
	qaResp, _ := agent.Send(context.Background(), qaPrompt)
	if !strings.Contains(qaResp, "agent-bridge signal QA_PASSED true") {
		t.Errorf("Expected QA_PASSED signal for QA prompt, got: %s", qaResp)
	}

	// 5. Test Manager Review
	managerPrompt := "I am the Manager Agent. Reviewing the project."
	managerResp, _ := agent.Send(context.Background(), managerPrompt)
	if !strings.Contains(managerResp, "agent-bridge signal PROJECT_SIGNED_OFF true --privileged") {
		t.Errorf("Expected PROJECT_SIGNED_OFF signal for manager prompt, got: %s", managerResp)
	}
}

func TestMockAgent_SmokeTest_Reproduction(t *testing.T) {
	agent := NewMockAgent()

	// This prompt simulates what we see in the logs
	// It contains "prime" (in "primes"), "python" (likely in history part of prompt), and "nothing to commit"
	prompt := `
You are the Coding Agent.
Task: Implement Prime Number Python Script.
History:
User: Write a python script to calculate prime numbers.
Agent: ...
User: Output:
Found 1229 primes
On branch agent/MFLP-12586
Your branch is up to date with 'origin/agent/MFLP-12586'.

nothing to commit, working tree clean
Everything up-to-date
`
	resp, _ := agent.Send(context.Background(), prompt)

	// We EXPECT the completion response (Heuristic 4)
	if strings.Contains(resp, "cat << 'EOF' > primes.py") {
		t.Errorf("FAILURE: Agent triggered Execution Phase (looping) instead of Completion Phase")
	}

	if !strings.Contains(resp, "agent-bridge feature set") {
		t.Errorf("Expected completion command, got: %s", resp)
	}
}

func TestTruncateString(t *testing.T) {
	s := "hello world"
	if truncateString(s, 5) != "hello" {
		t.Errorf("Expected 'hello', got '%s'", truncateString(s, 5))
	}
	if truncateString(s, 20) != "hello world" {
		t.Errorf("Expected 'hello world', got '%s'", truncateString(s, 20))
	}
}
