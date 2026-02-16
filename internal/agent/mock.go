package agent

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"
	"sync"
)

// MockAgent is a simple mock agent for testing and mock mode
// It returns predefined responses without making actual API calls
type MockAgent struct {
	responsePrefix string
	forcedResponse string
	// Internal state to track generated files
	generatedFiles map[string]bool
	mu             sync.Mutex
}

// NewMockAgent creates a new mock agent
func NewMockAgent() *MockAgent {
	return &MockAgent{
		responsePrefix: "Mock agent response",
		generatedFiles: make(map[string]bool),
	}
}

// SetResponse forces a specific response from the agent
func (m *MockAgent) SetResponse(response string) {
	m.forcedResponse = response
}

// Send implements the Agent interface
// It returns a mock response that acknowledges the prompt
func (m *MockAgent) Send(ctx context.Context, prompt string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.forcedResponse != "" {
		return m.forcedResponse, nil
	}

	lowerPrompt := strings.ToLower(prompt)

	// Helper to extract Ticket ID
	extractTicketID := func(p string) string {
		re := regexp.MustCompile(`(MFLP|TASK)-\d+`)
		matches := re.FindStringSubmatch(p)
		if len(matches) > 0 {
			return matches[0]
		}
		return "TASK-1" // Default fallback
	}

	// Heuristic for TPM / Ticket Generation
	// If the prompt asks for a ticket plan or identifies as a TPM, return a valid JSON list of tickets
	if strings.Contains(lowerPrompt, "technical program manager") || strings.Contains(lowerPrompt, "generate a ticket plan") {
		ticketID := extractTicketID(prompt)

		// Return a bash command to save features.json and import it
		// This ensures that the feature exists in the DB for the Runner to update later
		return fmt.Sprintf(`I have generated the plan.

` + "```bash" + `
cat <<EOF > features.json
{
  "project_name": "%s",
  "features": [
    {
      "id": "%s",
      "category": "Core",
      "priority": "MVP",
      "title": "Task %s",
      "description": "Implement the requirements for %s",
      "type": "Task",
      "status": "Open",
      "dependencies": {
        "depends_on_ids": [],
        "exclusive_write_paths": [],
        "read_only_paths": []
      }
    }
  ]
}
EOF
cat features.json | agent-bridge import
` + "```" + `
`, ticketID, ticketID, ticketID, ticketID), nil
	}

	// Heuristic for Primes coding task (Agent execution)
	if strings.Contains(lowerPrompt, "prime") || strings.Contains(lowerPrompt, "primes.json") || strings.Contains(os.Getenv("RECAC_INJECTED_FEATURES"), "prime") {

		ticketID := extractTicketID(prompt)

		// Helper to construct response
		makeResponse := func(content string) string {
			return fmt.Sprintf("I will proceed with the task.\n\n```bash\n%s\n```\n", content)
		}

		// 1. Check for "Test" task (Write Unit Tests)
		if strings.Contains(lowerPrompt, "test") && (strings.Contains(lowerPrompt, "write") || strings.Contains(lowerPrompt, "create")) {
			// Check if tests already exist
			if m.generatedFiles["test_primes.py"] {
				// Already exists, mark as done
				if ticketID != "" {
					return makeResponse(fmt.Sprintf("agent-bridge feature set %s --status done --passes true", ticketID)), nil
				}
				return makeResponse("echo 'Tests already exist.'"), nil
			}

			// Mark as generated
			m.generatedFiles["test_primes.py"] = true

			// Create tests
			return makeResponse(`cat <<EOF > test_primes.py
import unittest
import json
from primes import is_prime, generate_primes

class TestPrimes(unittest.TestCase):
    def test_is_prime(self):
        self.assertFalse(is_prime(1))
        self.assertTrue(is_prime(2))
        self.assertTrue(is_prime(3))
        self.assertFalse(is_prime(4))
        self.assertTrue(is_prime(5))

    def test_generate_primes(self):
        self.assertEqual(generate_primes(5), [2, 3, 5, 7, 11])

if __name__ == '__main__':
    unittest.main()
EOF`), nil
		}

		// 2. Check for "Implementation" task (Implement Prime Generator)
		// Default case if "implement" or "create" is present
		if strings.Contains(lowerPrompt, "implement") || strings.Contains(lowerPrompt, "create") || strings.Contains(lowerPrompt, "write") {
			// Check if implementation already exists
			if m.generatedFiles["primes.py"] {
				// Already exists, mark as done
				if ticketID != "" {
					return makeResponse(fmt.Sprintf("agent-bridge feature set %s --status done --passes true", ticketID)), nil
				}
				return makeResponse("echo 'Implementation already exists.'"), nil
			}

			// Mark as generated
			m.generatedFiles["primes.py"] = true

			// Create implementation
			return makeResponse(`cat <<EOF > primes.py
def is_prime(n):
    if n <= 1:
        return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0:
            return False
    return True

def generate_primes(n):
    primes = []
    num = 2
    while len(primes) < n:
        if is_prime(num):
            primes.append(num)
        num += 1
    return primes

if __name__ == "__main__":
    import json
    print(json.dumps(generate_primes(10)))
EOF`), nil
		}
	}

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
