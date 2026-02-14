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
	// IMPORTANT: Do NOT trigger this during Initializer or TPM phases, only for Coding/Manager
	if (strings.Contains(promptLower, "nothing to commit") || strings.Contains(promptLower, "clean")) &&
		!strings.Contains(promptLower, "initializer") &&
		!strings.Contains(promptLower, "generate-from-spec") {
		return "It seems there are no changes to commit. I will mark the task as complete.\nPROJECT_SIGNED_OFF", nil
	}

	// 2. TPM Agent (Jira Generation) - High Priority
	// Must come before Coding/Initializer heuristics because "primes" might be in the spec text.
	// Trigger: "generate-from-spec" or "create jira tickets" or if it looks like a spec analysis request.
	if strings.Contains(promptLower, "generate-from-spec") ||
		strings.Contains(promptLower, "create jira tickets") ||
		(strings.Contains(promptLower, "primes") && (strings.Contains(promptLower, "spec") || strings.Contains(promptLower, "ticket"))) {
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

	// 3. Initializer Agent (Feature Import)
	// Match "initializer agent" or generic init requests
	if strings.Contains(promptLower, "initializer agent") ||
		(strings.Contains(promptLower, "initialize") && strings.Contains(promptLower, "project")) {
		// Return a bash script to import features
		features := db.FeatureList{
			ProjectName: "recac-e2e",
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
		jsonStr = strings.ReplaceAll(jsonStr, "'", "'\\''")

		return fmt.Sprintf(`
#!/bin/bash
echo "Importing features..."
cat << 'EOF' | agent-bridge import
%s
EOF
`, jsonStr), nil
	}

	// 4. Manager Agent (Feature Verification)
	if strings.Contains(promptLower, "manager agent") || strings.Contains(promptLower, "manager") {
		return `
I have verified the work. It looks correct.
Running verification...

` + "```bash" + `
agent-bridge feature set "prime-script" --status passed
` + "```" + `

PROJECT_SIGNED_OFF
`, nil
	}

	// 5. Coding Agent (Primes Scenario)
	// Trigger on "primes" etc. but handled LAST to avoid catching spec/TPM prompts.
	if strings.Contains(promptLower, "primes") || strings.Contains(promptLower, "prime number script") || strings.Contains(promptLower, "req-script-runs-without-errors") {
		// Return a Python script that writes primes.json
		// Explicitly create the file to ensure git picks it up
		return `
Here is the python script to calculate primes:

` + "```bash" + `
cat << 'EOF' > primes.py
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
EOF

python3 primes.py
` + "```" + `
`, nil
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
