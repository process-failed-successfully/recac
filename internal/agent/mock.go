package agent

import (
	"context"
	"fmt"
	"strings"
	"time"
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
func (m *MockAgent) Send(ctx context.Context, prompt string) (string, error) {
	if m.forcedResponse != "" {
		return m.forcedResponse, nil
	}

	// Heuristic Role Detection

	// 1. TPM Role (Ticket Generation)
	if strings.Contains(prompt, "You are an expert Technical Program Manager (TPM)") || strings.Contains(prompt, "ID:[PRIMES]") {
		// Try to extract Repo URL from the prompt
		repo := "https://github.com/example/repo"
		// Simple extraction logic: Look for "Repo: <url>" or just use a known one if passed
		// In E2E tests, the repo URL is often passed in the prompt context
		if strings.Contains(prompt, "Repo: ") {
			// Extract URL after "Repo: "
			parts := strings.Split(prompt, "Repo: ")
			if len(parts) > 1 {
				// Take the first word after "Repo: "
				urlParts := strings.Fields(parts[1])
				if len(urlParts) > 0 {
					repo = urlParts[0]
				}
			}
		}

		// Return JSON ticket plan for Prime Python Scenario
		// The prompt contains "ID:[PRIMES]"
		return fmt.Sprintf(`
[
  {
    "title": "ID:[PRIMES] Create Prime Number Script",
    "description": "Implement a python script named 'primes.py' that calculates all prime numbers less than 10,000 and outputs them to a file named 'primes.json'. Repo: %s",
    "type": "Task",
    "children": []
  }
]
`, repo), nil
	}

	// 2. Initializer Agent
	// Check for role name OR generic initialization prompt (common when features are missing)
	if strings.Contains(strings.ToLower(prompt), "initializer agent") ||
		(strings.Contains(strings.ToLower(prompt), "initialize") && strings.Contains(strings.ToLower(prompt), "feature")) {
		// Just return a success message or simple bash
		// Note: The Initializer Agent is expected to import features into the DB
		// via agent-bridge import. The smoke test might fail if features aren't present.
		// We return a script that simulates importing a basic feature.
		return `
I will initialize the repository and register the features.

` + "```bash" + `
echo "Initializing repository..."
# Create an empty primes.py to start (optional)
touch primes.py

# Simulate feature import for smoke test
cat << 'EOF' | agent-bridge import
{
  "features": [
    {
      "id": "prime-script",
      "category": "functional",
      "priority": "MVP",
      "description": "Calculate primes",
      "status": "pending",
      "steps": ["Run primes.py"],
      "passes": false
    }
  ]
}
EOF
` + "```" + `
`, nil
	}

	// 3. Coding Agent
	if strings.Contains(prompt, "## YOUR ROLE - CODING AGENT") {
		// Check for specific task context
		if strings.Contains(prompt, "primes.py") || strings.Contains(prompt, "prime") || strings.Contains(prompt, "PRIMES") {
			return `
I will implement the prime number script as requested.

` + "```bash" + `
cat << 'EOF' > primes.py
import json

def is_prime(n):
    if n <= 1: return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0: return False
    return True

primes = [i for i in range(10000) if is_prime(i)]

with open('primes.json', 'w') as f:
    json.dump({"primes": primes}, f)

print(f"Generated {len(primes)} primes.")
EOF

python3 primes.py
` + "```" + `
`, nil
		}
	}

	// 4. QA Agent / Manager
	// The QA agent prompt header is likely ## YOUR ROLE - QA AGENT or similar
	if strings.Contains(prompt, "QA") || strings.Contains(prompt, "Manager") || strings.Contains(prompt, "Review") {
		// Return approval
		return `
LGTM. The code looks correct and meets the requirements.
`, nil
	}

	// Default Fallback
	response := fmt.Sprintf("%s:\n\nI received your prompt (%d characters). In mock mode, I would process this request and provide a response. The actual implementation would call the AI provider API here.\n\nPrompt preview: %s...",
		m.responsePrefix, len(prompt), truncateString(prompt, 100))
	return response, nil
}

// SendStream implements the Agent interface
func (m *MockAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	resp, err := m.Send(ctx, prompt)
	if err == nil && onChunk != nil {
		// Simulate streaming with a few chunks
		chunkSize := len(resp) / 5
		if chunkSize == 0 {
			chunkSize = len(resp)
		}
		for i := 0; i < len(resp); i += chunkSize {
			end := i + chunkSize
			if end > len(resp) {
				end = len(resp)
			}
			onChunk(resp[i:end])
			time.Sleep(10 * time.Millisecond) // Slight delay to simulate stream
		}
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
