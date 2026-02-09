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
func (m *MockAgent) Send(ctx context.Context, prompt string) (string, error) {
	if m.forcedResponse != "" {
		return m.forcedResponse, nil
	}

	// --- Heuristics for E2E Tests ---

	// 1. Initializer (Feature List Generation)
	if strings.Contains(prompt, "Create a feature list") || strings.Contains(prompt, "Generate feature list") {
		return `cat << 'EOF' > feature_list.json
[
  {
    "id": "req-setup-repo",
    "category": "core",
    "description": "Initialize repository",
    "status": "pending",
    "priority": "critical"
  },
  {
    "id": "req-implement-primes",
    "category": "core",
    "description": "Implement primes.py",
    "status": "pending",
    "priority": "critical"
  },
  {
    "id": "req-implement-tests",
    "category": "core",
    "description": "Implement test_primes.py",
    "status": "pending",
    "priority": "critical"
  }
]
EOF
agent-bridge import < feature_list.json
`, nil
	}

	// 2. TPM Agent (Ticket Generation for Jira)
	// Triggered by "Technical Program Manager" role header or "ROLE - TECHNICAL PROGRAM MANAGER"
	if strings.Contains(prompt, "Technical Program Manager") || strings.Contains(prompt, "ROLE - TECHNICAL PROGRAM MANAGER") {
		// [PRIMES] Scenario
		if strings.Contains(prompt, "[PRIMES]") || strings.Contains(prompt, "primes.py") {
			return `[
  {
    "title": "ID:[PRIMES] Prime Number Script",
    "description": "Implement a python script named 'primes.py' that calculates all prime numbers less than 10,000 and outputs them to a file named 'primes.json'.",
    "type": "Task",
    "children": [
        {
            "title": "ID:[req-setup-repo] Setup Repo",
            "description": "Initialize git",
            "type": "Subtask"
        },
        {
            "title": "ID:[req-implement-primes] Implement Primes",
            "description": "Write primes.py",
            "type": "Subtask"
        },
        {
            "title": "ID:[req-implement-tests] Implement Tests",
            "description": "Write test_primes.py",
            "type": "Subtask"
        }
    ]
  }
]`, nil
		}
		// Default TPM response
		return `[{"title": "ID:[DEFAULT] Default Task", "description": "Default task description", "type": "Task"}]`, nil
	}

	// 3. Coding Agent
	// Triggered by "CODING AGENT" or "recac-agent" context
	if strings.Contains(prompt, "CODING AGENT") || strings.Contains(prompt, "recac-agent") || strings.Contains(prompt, "ROLE - CODING AGENT") {
		// Feature: Setup Repo
		if strings.Contains(prompt, "req-setup-repo") {
			return "git init\ngit add .\ngit commit -m \"Initial commit\"", nil
		}
		// Feature: Implement Primes
		if strings.Contains(prompt, "req-implement-primes") || strings.Contains(prompt, "primes.py") {
			return `cat << 'EOF' > primes.py
import json

def calculate_primes(n):
    primes = []
    for possiblePrime in range(2, n):
        isPrime = True
        for num in range(2, int(possiblePrime ** 0.5) + 1):
            if possiblePrime % num == 0:
                isPrime = False
                break
        if isPrime:
            primes.append(possiblePrime)
    return primes

def write_primes_to_json(primes, filename):
    with open(filename, 'w') as f:
        json.dump({'primes': primes}, f)

if __name__ == "__main__":
    n = 10000
    primes = calculate_primes(n)
    write_primes_to_json(primes, 'primes.json')
EOF
`, nil
		}
		// Feature: Implement Tests
		if strings.Contains(prompt, "req-implement-tests") || strings.Contains(prompt, "test_primes.py") {
			return `cat << 'EOF' > test_primes.py
import unittest
import json
from primes import calculate_primes

class TestPrimes(unittest.TestCase):
    def test_primes_length(self):
        primes = calculate_primes(10000)
        self.assertEqual(len(primes), 1229)

if __name__ == "__main__":
    unittest.main()
EOF
python3 test_primes.py
`, nil
		}
	}

	// 4. QA Agent
	if strings.Contains(prompt, "QA AGENT") {
		return "agent-bridge signal QA_PASSED true", nil
	}

	// 5. Project Manager (Review/Signoff)
	if strings.Contains(prompt, "PROJECT MANAGER") {
		return "agent-bridge signal --privileged PROJECT_SIGNED_OFF true", nil
	}

	// Default response
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

// SendImage implements the VisionAgent interface
func (m *MockAgent) SendImage(ctx context.Context, prompt string, imagePath string) (string, error) {
	if m.forcedResponse != "" {
		return m.forcedResponse, nil
	}
	return fmt.Sprintf("%s (Vision):\n\nI received your prompt and image '%s'.", m.responsePrefix, imagePath), nil
}

// truncateString truncates a string to a maximum length
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}
