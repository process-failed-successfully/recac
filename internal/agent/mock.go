package agent

import (
	"context"
	"fmt"
	"strings"
)

// MockAgent is a smart mock agent for testing and mock mode
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

	// 1. Initializer Agent
	// Detect "Initializer" role or "git init" intent
	if strings.Contains(prompt, "Initializer") || strings.Contains(prompt, "INITIALIZER") {
		return m.initializerResponse(), nil
	}

	// 2. TPM Agent (Plan)
	// Detect "Technical Program Manager" or "TPM"
	if strings.Contains(prompt, "Technical Program Manager") || strings.Contains(prompt, "TPM") {
		if strings.Contains(prompt, "[PRIMES]") {
			return m.primesPlanResponse(), nil
		}
		// Default plan
		return m.defaultPlanResponse(), nil
	}

	// 3. Coding Agent
	// Detect "Developer" or "Coding Agent"
	if strings.Contains(prompt, "Developer") || strings.Contains(prompt, "Coding Agent") {
		lowerPrompt := strings.ToLower(prompt)
		// Detect "primes" intent
		if strings.Contains(lowerPrompt, "prime") || strings.Contains(prompt, "[PRIMES]") {
			return m.primesImplementationResponse(), nil
		}
	}

	// 4. QA Agent
	if strings.Contains(prompt, "QA Agent") {
		return "## QA Report\n\nAll tests passed.", nil
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

func (m *MockAgent) initializerResponse() string {
	// Returns a script to initialize the repo and run agent-bridge import
	// Use standard markdown code block
	return "Here is the initialization script:\n\n```bash\n#!/bin/bash\ngit init\ngit config user.email \"bot@recac.com\"\ngit config user.name \"Recac Bot\"\necho \"# Project\" > README.md\ngit add .\ngit commit -m \"Initial commit\" || true\nagent-bridge import\n```"
}

func (m *MockAgent) primesPlanResponse() string {
	// Returns the JSON ticket list for the PRIMES scenario
	return `[
  {
    "id": "PRIMES",
    "title": "Calculate Primes",
    "description": "Create a script primes.py that calculates prime numbers up to 100 and saves them to primes.json. [PRIMES]",
    "type": "Task",
    "status": "Open",
    "assigned_to": "Developer"
  }
]`
}

func (m *MockAgent) defaultPlanResponse() string {
	return `[
  {
    "id": "TASK-1",
    "title": "Default Task",
    "description": "This is a default task from the mock agent.",
    "type": "Task",
    "status": "Open",
    "assigned_to": "Developer"
  }
]`
}

func (m *MockAgent) primesImplementationResponse() string {
	// Returns the implementation script for primes.py
	// Must use cat <<EOF for file creation and run python3
	return `Here is the implementation:

` + "```bash" + `
#!/bin/bash
cat <<EOF > primes.py
import json

def is_prime(n):
    if n <= 1: return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0: return False
    return True

primes = [x for x in range(1, 101) if is_prime(x)]
print(primes)

with open('primes.json', 'w') as f:
    json.dump(primes, f)
EOF

python3 primes.py
git add primes.py primes.json
git commit -m "Implement primes calculation" --author="Recac Bot <bot@recac.com>"
agent-bridge feature set PRIMES implemented
` + "```"
}
