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

	// Expect ID:[PRIMES] in the Title
	if !strings.Contains(planResp, "\"title\": \"ID:[PRIMES]") {
		t.Errorf("Expected ID:[PRIMES] in ticket title for mapping, got: %s", planResp)
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
	if !strings.Contains(compResp, "agent-bridge feature set \"PRIMES\" --status done") {
		t.Errorf("Expected completion command for 'nothing to commit', got: %s", compResp)
	}
	// Check for robust import
	if !strings.Contains(compResp, "agent-bridge import") {
		t.Errorf("Expected agent-bridge import command for robustness, got: %s", compResp)
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

func TestMockAgent_Stateful_PrimeScenario(t *testing.T) {
	agent := NewMockAgent()

	// 1. First call: Should generate code
	prompt1 := "Write a python script to calculate prime numbers."
	resp1, _ := agent.Send(context.Background(), prompt1)

	if !strings.Contains(resp1, "cat << 'EOF' > primes.py") {
		t.Errorf("First call expected python code generation, got: %s", resp1)
	}
	if agent.primeCalls != 1 {
		t.Errorf("Expected primeCalls to be 1, got %d", agent.primeCalls)
	}

	// 2. Second call: Should complete (even without 'nothing to commit' explicit context if truncated,
	// because we rely on state)
	prompt2 := "Write a python script to calculate prime numbers. (Iteration 2)"
	resp2, _ := agent.Send(context.Background(), prompt2)

	if strings.Contains(resp2, "cat << 'EOF' > primes.py") {
		t.Errorf("Second call failure: Agent looped execution phase instead of completion")
	}
	if !strings.Contains(resp2, "agent-bridge feature set") {
		t.Errorf("Second call expected completion command, got: %s", resp2)
	}
}

func TestMockAgent_Stateful_PrimeScenario_Loop(t *testing.T) {
	agent := NewMockAgent()
	agent.primeCalls = 1 // Set state to indicate code already generated

	// Scenario: Git output contains filename but not 'python' keyword
	// Example: "create mode 100644 primes.py"
	prompt := "Command Output:\n[master 1234567] Add primes script\n 1 file changed, 10 insertions(+)\n create mode 100644 primes.py"

	resp, _ := agent.Send(context.Background(), prompt)

	// Should trigger completion because we are in state 1 and context relates to primes.py
	if !strings.Contains(resp, "agent-bridge feature set") {
		t.Errorf("Expected completion command for 'primes.py' context, got default response: %s", resp)
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
