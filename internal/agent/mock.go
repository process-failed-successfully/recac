package agent

import (
	"context"
	"fmt"
	"strings"
)

// MockAgent is a smart mock agent for testing and mock mode
// It returns predefined responses based on prompt heuristics to simulate a real agent flow
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

	promptLower := strings.ToLower(prompt)

	// --- 1. QA Agent ---
	// Triggers when the prompt indicates the role is QA Agent
	if strings.Contains(promptLower, "your role - qa agent") || strings.Contains(promptLower, "qa agent") {
		return `
I have verified the changes and they look correct. All tests passed.

` + "```bash" + `
agent-bridge signal QA_PASSED true
` + "```", nil
	}

	// --- 2. Project Manager ---
	// Triggers when the prompt indicates the role is Project Manager or Manager Review
	if strings.Contains(promptLower, "your role - project manager") || strings.Contains(promptLower, "project manager") || strings.Contains(promptLower, "manager review") {
		return `
The project is complete and meets all requirements.

` + "```bash" + `
agent-bridge signal PROJECT_SIGNED_OFF true --privileged
` + "```", nil
	}

	// --- 3. Initializer Agent ---
	// Triggers when the prompt asks to initialize features
	if strings.Contains(promptLower, "initializer agent") || (strings.Contains(promptLower, "initialize") && strings.Contains(promptLower, "feature")) {
		return `
I will initialize the feature requirements in the database.

` + "```bash" + `
cat <<EOF | agent-bridge import
{
  "features": [
    {
      "id": "req-script-runs-without-errors",
      "description": "The script runs without errors",
      "priority": "critical",
      "category": "functional"
    },
    {
      "id": "req-primes-json-created",
      "description": "The output file primes.json is created",
      "priority": "critical",
      "category": "functional"
    }
  ]
}
EOF
` + "```", nil
	}

	// --- 4. Technical Program Manager (Planning) ---
	// Triggers for the planning phase
	if strings.Contains(promptLower, "technical program manager") || strings.Contains(prompt, "ticketNode") {
		return `
I have analyzed the requirements and created a plan.

` + "```json" + `
[
  {
    "title": "Implement Prime Number Script",
    "description": "Create a Python script to calculate primes up to 10,000",
    "type": "Task",
    "children": []
  }
]
` + "```", nil
	}

	// Important: Check for role-specific prompts BEFORE general project keywords
	// to prevent 'Coding Agent' heuristic from capturing QA or Manager prompts that
	// contain project details in context.

	// --- 5. Coding Agent (Prime Python Scenario) ---
	// Triggers when the prompt mentions primes.py or the project ID, BUT only if it's NOT a review task
	isReview := strings.Contains(promptLower, "qa agent") || strings.Contains(promptLower, "project manager") || strings.Contains(promptLower, "manager review")
	if !isReview && (strings.Contains(promptLower, "primes.py") || strings.Contains(prompt, "ID:[PRIMES]") || strings.Contains(prompt, "1229")) {

		// If git status shows clean, we are done
		if strings.Contains(promptLower, "nothing to commit") || strings.Contains(promptLower, "working tree clean") {
			return `
The work is complete and committed.

` + "```bash" + `
agent-bridge feature set req-script-runs-without-errors completed
agent-bridge feature set req-primes-json-created completed
` + "```", nil
		}

		return `
I will implement the prime number script as requested.

` + "```bash" + `
# Create the python script
cat <<EOF > primes.py
import json

def is_prime(n):
    if n <= 1:
        return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0:
            return False
    return True

primes = [x for x in range(10000) if is_prime(x)]
print(f"Found {len(primes)} prime numbers.")

with open('primes.json', 'w') as f:
    json.dump({"primes": primes}, f)
EOF

# Run the script to generate output
python3 primes.py

# Commit changes
git add primes.py primes.json
git commit -m "Implement prime number script"
` + "```", nil
	}

	// Default response if no heuristic matches
	return fmt.Sprintf("%s:\n\nI received your prompt (%d characters). In mock mode, I would process this request and provide a response. The actual implementation would call the AI provider API here.\n\nPrompt preview: %s...",
		m.responsePrefix, len(prompt), truncateString(prompt, 100)), nil
}

// SendStream implements the Agent interface
func (m *MockAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	resp, err := m.Send(ctx, prompt)
	if err == nil && onChunk != nil {
		onChunk(resp)
	}
	return resp, err
}

// truncateString truncates a string to a maximum length
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}
