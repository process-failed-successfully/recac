package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"recac/internal/db"
)

// MockAgent is a smart mock agent for E2E scenarios
type MockAgent struct {
	responsePrefix string
	forcedResponse string
}

// NewMockAgent creates a new mock agent
func NewMockAgent() *MockAgent {
	return &MockAgent{
		responsePrefix: "Mock Agent",
	}
}

// SetResponse forces a specific response from the agent
func (m *MockAgent) SetResponse(response string) {
	m.forcedResponse = response
}

// Send implements the Agent interface with heuristics for E2E scenarios
func (m *MockAgent) Send(ctx context.Context, prompt string) (string, error) {
	if m.forcedResponse != "" {
		return m.forcedResponse, nil
	}

	promptLower := strings.ToLower(prompt)

	// 1. Completion Heuristic (Prevent No-Op Loop)
	// Checks for git status indicating nothing to commit
	if strings.Contains(promptLower, "nothing to commit") || strings.Contains(promptLower, "clean") {
		return "It seems there are no changes to commit. I will mark the task as complete.\nPROJECT_SIGNED_OFF", nil
	}

	// 2. Initializer Agent (Feature Import)
	// Must come before Coding Agent to avoid false positives if prompt mentions both
	if strings.Contains(promptLower, "initializer agent") {
		// Return a bash script to import features
		// We use a bash script that pipes JSON to agent-bridge import
		// The JSON must match the db.FeatureList structure
		features := db.FeatureList{
			ProjectName: "recac-e2e", // Dummy project name, will be overridden or ignored
			Features: []db.Feature{
				{
					ID:          "prime-script",
					Description: "A script that calculates prime numbers",
					Status:      "pending",
				},
			},
		}

		jsonBytes, _ := json.Marshal(features)
		jsonStr := string(jsonBytes)

		// Escape single quotes for bash
		jsonStr = strings.ReplaceAll(jsonStr, "'", "'\\''")

		return fmt.Sprintf(`
#!/bin/bash
cat << 'EOF' | agent-bridge import
%s
EOF
`, jsonStr), nil
	}

	// 3. Coding Agent (Primes Scenario)
	// Trigger on "primes" or "prime number script" but NOT "initializer agent"
	// Also handles "req-script-runs-without-errors" keyword if present in prompt
	if (strings.Contains(promptLower, "primes") || strings.Contains(promptLower, "prime number script") || strings.Contains(promptLower, "req-script-runs-without-errors")) && !strings.Contains(promptLower, "initializer agent") {
		// Return a Python script that writes primes.json
		return `
Here is the python script to calculate primes:

` + "```python" + `
import json

def is_prime(n):
    if n <= 1:
        return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0:
            return False
    return True

primes = [x for x in range(2, 100) if is_prime(x)]
with open("primes.json", "w") as f:
    json.dump({"primes": primes}, f)
` + "```" + `
`, nil
	}

	// 4. Manager Agent (Feature Verification)
	if strings.Contains(promptLower, "manager agent") || strings.Contains(promptLower, "manager") {
		// Mark feature as passed
		return `
I have verified the work. It looks correct.
Running verification...

` + "```bash" + `
agent-bridge feature set "prime-script" --status passed
` + "```" + `

PROJECT_SIGNED_OFF
`, nil
	}

	// 5. TPM Agent (Jira Generation)
	// Trigger: "generate-from-spec" or "create jira tickets" or just generic spec processing
	// We assume if it's none of the above and contains "primes" or "spec", it's likely the TPM phase.
	// The runner calls this with the spec content.
	if strings.Contains(promptLower, "generate-from-spec") || (strings.Contains(promptLower, "primes") && (strings.Contains(promptLower, "spec") || strings.Contains(promptLower, "ticket"))) {
		tickets := []map[string]string{
			{
				"title":       "Implement Primes ID:[PRIMES]",
				"type":        "Task",
				"description": "Create a python script that calculates prime numbers and writes them to primes.json.",
			},
		}
		jsonBytes, _ := json.Marshal(tickets)
		return string(jsonBytes), nil
	}

	// Default fallback
	return fmt.Sprintf("Mock Agent received prompt (%d chars). No heuristic matched.", len(prompt)), nil
}


// SendStream implements the Agent interface
func (m *MockAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	resp, err := m.Send(ctx, prompt)
	if err == nil && onChunk != nil {
		onChunk(resp)
	}
	return resp, err
}
