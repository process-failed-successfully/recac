package agent

import (
	"context"
	"fmt"
	"strings"
)

// MockAgent is a simple mock agent for testing and mock mode
// It returns predefined responses without making actual API calls
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
// It returns a mock response that acknowledges the prompt
func (m *MockAgent) Send(ctx context.Context, prompt string) (string, error) {
	if m.forcedResponse != "" {
		return m.forcedResponse, nil
	}

	// Log the prompt for debugging E2E tests
	fmt.Printf("DEBUG: MockAgent Prompt: %s\n", truncateString(prompt, 200))

	upperPrompt := strings.ToUpper(prompt)

	// --- 1. INITIALIZER AGENT ---
	if strings.Contains(upperPrompt, "ROLE - INITIALIZER AGENT") && !strings.Contains(upperPrompt, "YOUR ROLE - CODING AGENT") {
		return `I have analyzed the spec.

` + "```bash" + `
cat <<EOF > feature_list.json
{
  "project_name": "primes",
  "features": [
    {
      "id": "PRIMES",
      "description": "Script calculates primes correctly",
      "status": "pending",
      "steps": ["Run script", "Check output"]
    }
  ]
}
EOF

# Import the features into the database
agent-bridge import < feature_list.json
` + "```" + `
`, nil
	}

	// --- 2. PLANNER (Lead Software Architect) ---
	if strings.Contains(upperPrompt, "LEAD SOFTWARE ARCHITECT") || strings.Contains(upperPrompt, "ROLE - PLANNER") {
		return `{
"project_name": "primes",
"features": [
  {
    "id": "PRIMES",
    "category": "functional",
    "description": "Script calculates primes correctly",
    "status": "pending",
    "steps": [
      "Step 1: Create primes.py",
      "Step 2: Implement sieve of eratosthenes",
      "Step 3: Verify output for n=10"
    ],
    "dependencies": {}
  }
]
}`, nil
	}

	// --- 3. TECHNICAL PROGRAM MANAGER (TPM) ---
	if strings.Contains(upperPrompt, "TECHNICAL PROGRAM MANAGER") || strings.Contains(upperPrompt, "ROLE - TPM") {
		return `[
  {
    "id": "PRIMES",
    "title": "ID:[PRIMES] Primes Script",
    "type": "task",
    "status": "todo",
    "description": "Implement primes.py",
    "dependencies": []
  }
]`, nil
	}

	// --- 4. CODING AGENT (Developer) ---
	// Heuristic: Check for "CODING AGENT" or keywords related to the primes scenario
	if strings.Contains(upperPrompt, "CODING AGENT") ||
	   (strings.Contains(upperPrompt, "PRIMES") && strings.Contains(upperPrompt, "CALCULATE")) ||
	   strings.Contains(prompt, "primes.py") {

		// Guard against infinite loops if already implemented
		if strings.Contains(upperPrompt, "IMPLEMENTED") && strings.Contains(upperPrompt, "PASSES: TRUE") {
             // In a real scenario, we might want to just say "done".
             // But for now, let's behave as if we are doing it.
		}

		return `
I will implement the primes script.

` + "```bash" + `
# Create the python script
cat <<EOF > primes.py
import json
import sys

def get_primes(n):
    primes = []
    for num in range(2, n):
        is_prime = True
        for i in range(2, int(num ** 0.5) + 1):
            if num % i == 0:
                is_prime = False
                break
        if is_prime:
            primes.append(num)
    return primes

if __name__ == "__main__":
    n = 10000
    result = {"primes": get_primes(n)}
    with open("primes.json", "w") as f:
        json.dump(result, f)
    print("Generated primes.json")
EOF

# Run the script to generate the json
python3 primes.py

# Force track the file
git add -f primes.py primes.json

# Mark feature as done
agent-bridge feature set PRIMES --status done --passes true
` + "```" + `
`, nil
	}

	// --- 5. QA AGENT ---
	if strings.Contains(upperPrompt, "ROLE - QA AGENT") {
		return "```bash\nagent-bridge signal QA_PASSED true\n```", nil
	}

	// --- 6. PROJECT MANAGER (Reviewer) ---
	if strings.Contains(upperPrompt, "PROJECT MANAGER") {
		// Always approve in mock mode
		return "```bash\nagent-bridge signal PROJECT_SIGNED_OFF true\n```", nil
	}

	// --- FALLBACK ---
	// Return a mock response that shows the agent received the prompt
	// This allows the session to run without requiring real API keys
	response := fmt.Sprintf("%s:\n\nI received your prompt (%d characters). In mock mode, I would process this request and provide a response. The actual implementation would call the AI provider API here.\n\nPrompt preview: %s...\n\n```bash\necho \"Mock Agent Default Response\"\n```",
		m.responsePrefix, len(prompt), truncateString(prompt, 100))
	return response, nil
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
