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
		// Return JSON ticket plan for Prime Python Scenario
		// The prompt contains "ID:[PRIMES]"
		return `
[
  {
    "title": "ID:[PRIMES] Create Prime Number Script",
    "description": "Implement a python script named 'primes.py' that calculates all prime numbers less than 10,000 and outputs them to a file named 'primes.json'.",
    "type": "Task",
    "children": []
  }
]
`, nil
	}

	// 2. Initializer Agent
	if strings.Contains(prompt, "INITIALIZER AGENT") || strings.Contains(prompt, "You are an Initializer Agent") {
		// Just return a success message or simple bash that creates feature list and imports it
		// Crucial: The prompt expects 'agent-bridge import' for features.
		return `
I will initialize the repository.

` + "```bash" + `
echo "Initializing repository..."
cat << 'EOF' | agent-bridge import --project "$RECAC_PROJECT_ID"
{
  "project_name": "Prime Script",
  "features": [
    {
      "id": "primes-script",
      "category": "functional",
      "priority": "MVP",
      "description": "Implement primes.py script",
      "status": "pending",
      "steps": ["Run python script", "Check output"],
      "passes": false
    }
  ]
}
EOF
` + "```" + `
`, nil
	}

	// 3. Coding Agent
	if strings.Contains(prompt, "CODING AGENT") || strings.Contains(prompt, "Coding Agent") {
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

# IMPORTANT: Mark feature as complete so we don't loop forever
agent-bridge feature set primes-script --status completed --passes true
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
