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
	// DEBUG: Print prompt to stdout to help debug CI failures where heuristics don't match
	fmt.Printf("[MOCK AGENT] Received prompt (%d chars): %s\n", len(prompt), truncateString(prompt, 200))

	if m.forcedResponse != "" {
		return m.forcedResponse, nil
	}

	// 1. TPM Role (Project Planning)
	// The CLI/Orchestrator expects a JSON array of tickets.
	// We detect this by checking for the TPM role description.
	if strings.Contains(prompt, "You are an expert Technical Program Manager (TPM)") {
		// Return a mock JSON array of tickets
		// This simulates a breakdown of tasks
		return `[
  {
    "title": "ID:[PRIMES] Implement Prime Number Checker",
    "description": "Create a Python script that checks if a number is prime.",
    "status": "Todo",
    "type": "Task"
  },
  {
    "title": "ID:[QA] Verify Prime Implementation",
    "description": "Run the prime number checker with test cases.",
    "status": "Todo",
    "type": "QA"
  }
]`, nil
	}

	// 2. Initializer Role (Feature Extraction)
	// The CLI might also use an "Initializer" agent to extract features first.
	// If the prompt asks for features extraction or similar (usually has JSON format instructions).
	// However, the smoke test seems to jump straight to TPM or Coding.
	// Let's add a basic check for "Initializer" just in case.
	if strings.Contains(prompt, "You are an Initializer Agent") {
		// Return a bash script that imports features into the DB using agent-bridge
		// Must be wrapped in markdown for the runner to detect it
		// We explicitly pass the project ID to ensure correct DB association
		return "```bash\n" + `echo "Importing features for project: $RECAC_PROJECT_ID"
cat << 'EOF' | agent-bridge import --project "$RECAC_PROJECT_ID"
{
  "features": [
    {"description": "Calculate prime numbers", "status": "pending"},
    {"description": "Handle invalid input", "status": "pending"}
  ]
}
EOF
echo "Import finished with code $?"
` + "\n```", nil
	}

	// 3. Coding Agent (Primes Task)
	// Detects the specific task or "Coding Agent" role.
	// The smoke test scenario is likely "prime-python".
	// We check for "PRIME" (case-insensitive), "Coding Agent", or "Python" script requests.
	upperPrompt := strings.ToUpper(prompt)
	if strings.Contains(upperPrompt, "PRIME") ||
	   strings.Contains(prompt, "Coding Agent") ||
	   strings.Contains(upperPrompt, "PYTHON") ||
	   strings.Contains(upperPrompt, "SCRIPT") {
		// Return a bash script to implement the prime checker
		return `cat << 'EOF' > primes.py
import sys

def is_prime(n):
    if n <= 1:
        return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0:
            return False
    return True

if __name__ == "__main__":
    if len(sys.argv) > 1:
        try:
            num = int(sys.argv[1])
            print(f"{num} is prime: {is_prime(num)}")
        except ValueError:
            print("Invalid input")
    else:
        print("Usage: python3 primes.py <number>")
EOF

# Verify it works
python3 primes.py 7
python3 primes.py 10

# Signal completion
agent-bridge signal PROJECT_SIGNED_OFF true --privileged || true
`, nil
	}

	// 4. QA/Manager Role
	// Detects "QA" or "Manager" to approve work.
	// Also checks for "Review" or "Verify" which are common in QA prompts.
	if strings.Contains(prompt, "QA") ||
	   strings.Contains(prompt, "Manager") ||
	   strings.Contains(prompt, "Review") ||
	   strings.Contains(strings.ToUpper(prompt), "VERIFY") {
		// Return a signal to approve
		return `QA_PASSED`, nil
	}

	// Default fallback
	// Return a mock response that shows the agent received the prompt
	// This allows the session to run without requiring real API keys
	response := fmt.Sprintf("%s:\n\nI received your prompt (%d characters). In mock mode, I would process this request and provide a response. The actual implementation would call the AI provider API here.\n\nPrompt preview: %s...",
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
