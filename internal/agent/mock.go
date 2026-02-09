package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// MockAgent is a heuristic-based mock agent for E2E testing
type MockAgent struct {
	responsePrefix string
	forcedResponse string
}

// NewMockAgent creates a new mock agent
func NewMockAgent() *MockAgent {
	return &MockAgent{
		responsePrefix: "Mock agent response",
	}
}

// SetResponse forces a specific response from the agent
func (m *MockAgent) SetResponse(response string) {
	m.forcedResponse = response
}

// Send implements the Agent interface
func (m *MockAgent) Send(ctx context.Context, prompt string) (string, error) {
	if m.forcedResponse != "" {
		return m.forcedResponse, nil
	}

	// === Heuristic 1: Technical Program Manager (TPM) ===
	// Detects TPM role and generates tickets for the [PRIMES] scenario
	if strings.Contains(prompt, "ROLE - TECHNICAL PROGRAM MANAGER") || strings.Contains(prompt, "Technical Program Manager") {
		type Ticket struct {
			Title              string   `json:"title"`
			ID                 string   `json:"id"`
			Description        string   `json:"description"`
			Type               string   `json:"type"`
			Status             string   `json:"status"`
			AcceptanceCriteria []string `json:"acceptance_criteria"`
			Priority           string   `json:"priority"`
		}

		tickets := []Ticket{
			{
				Title:       "Implement prime number script",
				ID:          "ID:[PRIMES]",
				Description: "Create a python script primes.py that prints prime numbers up to 10000.",
				Type:        "Task",
				Status:      "To Do",
				AcceptanceCriteria: []string{
					"Implement prime number script",
					"primes.py exists",
					"contains exactly 1229 primes",
				},
				Priority: "High",
			},
		}

		resp, _ := json.Marshal(tickets)
		return string(resp), nil
	}

	// === Heuristic 2: Initializer Agent ===
	// Detects Initializer role and creates feature_list.json
	if strings.Contains(prompt, "ROLE - INITIALIZER AGENT") || strings.Contains(prompt, "CREATE FEATURE_LIST.JSON") {
		script := "```bash\n" +
			"cat <<EOF > feature_list.json\n" +
			"[\n" +
			"  {\n" +
			"    \"id\": \"req-implement-prime-number-script\",\n" +
			"    \"description\": \"Implement prime number script\",\n" +
			"    \"status\": \"todo\"\n" +
			"  },\n" +
			"  {\n" +
			"    \"id\": \"req-primes-py-exists\",\n" +
			"    \"description\": \"primes.py exists\",\n" +
			"    \"status\": \"todo\"\n" +
			"  },\n" +
			"  {\n" +
			"    \"id\": \"req-contains-exactly-1229-primes\",\n" +
			"    \"description\": \"contains exactly 1229 primes\",\n" +
			"    \"status\": \"todo\"\n" +
			"  }\n" +
			"]\n" +
			"EOF\n" +
			"agent-bridge import < feature_list.json\n" +
			"```\n"
		return script, nil
	}

	// === Heuristic 3: Coding Agent ([PRIMES] Scenario) ===
	// Detects Coding role and implements the primes.py script
	// Checks for feature IDs or role header
	if strings.Contains(prompt, "ROLE - CODING AGENT") ||
		strings.Contains(prompt, "req-implement-prime-number-script") ||
		strings.Contains(prompt, "req-script-prints-primes") ||
		strings.Contains(prompt, "req-contains-exactly-1229-primes") ||
		strings.Contains(prompt, "[PRIMES]") {

		script := "```bash\n" +
			"cat <<EOF > primes.py\n" +
			"def is_prime(n):\n" +
			"    if n <= 1: return False\n" +
			"    for i in range(2, int(n**0.5) + 1):\n" +
			"        if n % i == 0:\n" +
			"            return False\n" +
			"    return True\n" +
			"\n" +
			"primes = [str(i) for i in range(10000) if is_prime(i)]\n" +
			"print(\", \".join(primes))\n" +
			"EOF\n" +
			"\n" +
			"# Verify output count (just for log)\n" +
			"python3 primes.py | tr ',' '\\n' | wc -l\n" +
			"\n" +
			"# Mark features as done\n" +
			"agent-bridge feature set req-implement-prime-number-script done || echo \"Feature not found\"\n" +
			"agent-bridge feature set req-primes-py-exists done || echo \"Feature not found\"\n" +
			"agent-bridge feature set req-contains-exactly-1229-primes done || echo \"Feature not found\"\n" +
			"\n" +
			"# Git commit (with error handling for idempotency)\n" +
			"git add primes.py\n" +
			"git commit -m \"Implement primes.py\" || echo \"Nothing to commit\"\n" +
			"```\n"
		return script, nil
	}

	// === Heuristic 4: QA Agent ===
	// Detects QA role and verifies the solution
	if strings.Contains(prompt, "ROLE - QA AGENT") {
		script := "```bash\n" +
			"# Run verification\n" +
			"COUNT=$(python3 primes.py | tr ',' '\\n' | wc -l)\n" +
			"if [ \"$COUNT\" -eq \"1229\" ]; then\n" +
			"    echo \"Verification Passed: 1229 primes found\"\n" +
			"    agent-bridge signal --privileged QA_PASSED true\n" +
			"else\n" +
			"    echo \"Verification Failed: Expected 1229, got $COUNT\"\n" +
			"    exit 1\n" +
			"fi\n" +
			"```\n"
		return script, nil
	}

	// === Heuristic 5: Project Manager (Sign Off) ===
	// Detects Project Manager role and signs off the project
	if strings.Contains(prompt, "ROLE - PROJECT MANAGER") || strings.Contains(prompt, "Manager Review") {
		// Ensure strict argument order for signal
		return "```bash\nagent-bridge signal --privileged PROJECT_SIGNED_OFF true\n```", nil
	}

	// === Heuristic 6: Loop Breaker / Safety Net ===
	// Detects if the agent is stuck in a loop (e.g., git commit failed because nothing changed)
	if strings.Contains(prompt, "nothing to commit") || strings.Contains(prompt, "working tree clean") {
		// Force move to next stage or finish
		return "```bash\nagent-bridge signal --privileged QA_PASSED true && agent-bridge signal --privileged PROJECT_SIGNED_OFF true\n```", nil
	}

	// Fallback for unknown prompts
	return fmt.Sprintf("I received your prompt (%d characters). I am a heuristic mock agent and did not recognize a specific scenario pattern.", len(prompt)), nil
}

// SendStream implements the Agent interface
func (m *MockAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	resp, err := m.Send(ctx, prompt)
	if err == nil && onChunk != nil {
		onChunk(resp)
	}
	return resp, err
}
