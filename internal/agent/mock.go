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

	lowerPrompt := strings.ToLower(prompt)

	// 1. Ticket Generation (Technical Program Manager)
	if containsTicketKeywords(lowerPrompt) || strings.Contains(lowerPrompt, "role - technical program manager") {
		// [PRIMES] Scenario
		if strings.Contains(prompt, "ID:[PRIMES]") || strings.Contains(prompt, "CRITICAL INSTRUCTION FOR TICKET GENERATION") {
			return `[
  {
    "id": "PRIMES",
    "title": "Implement Primes Script",
    "description": "Calculate primes < 10000",
    "type": "Task",
    "status": "Todo"
  }
]`, nil
		}
	}

	// 2. Feature Extraction (Architect)
	if strings.Contains(lowerPrompt, "role - lead software architect") || strings.Contains(lowerPrompt, "role: lead software architect") {
		// Return JSON features for agent-bridge import
		// For prime-python, we map the task to a feature
		return `{
   "project_name": "primes",
   "features": [
       {
           "id": "req-primes-implementation",
           "name": "Implement Primes Script",
           "description": "Create primes.py and primes.json",
           "priority": "1",
           "dependencies": {"depends_on_ids": []}
       }
   ]
}`, nil
	}

	// 3. Implementation (Coding Agent)
	// Heuristic: "Coding Agent" AND ("[PRIMES]" OR specific feature ID)
	if strings.Contains(lowerPrompt, "role - coding agent") || strings.Contains(lowerPrompt, "role: coding agent") || strings.Contains(lowerPrompt, "role: coding") {
		if strings.Contains(prompt, "PRIMES") || strings.Contains(lowerPrompt, "prime") || strings.Contains(lowerPrompt, "req-primes") {
			// Generate the script
			return `
#!/bin/bash
cat << 'EOF' > primes.py
import json

def is_prime(n):
    if n <= 1: return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0: return False
    return True

primes = [i for i in range(2, 10000) if is_prime(i)]
with open('primes.json', 'w') as f:
    json.dump({"primes": primes}, f)
EOF

python3 primes.py
git add -f primes.py primes.json
git commit -m "Implement primes calculation" || true
# Signal feature completion
agent-bridge feature set req-primes-implementation --status done --passes true
`, nil
		}
	}

	// 4. QA / Signoff
	if strings.Contains(lowerPrompt, "role - qa agent") || strings.Contains(lowerPrompt, "role: qa agent") {
		return "agent-bridge signal QA_PASSED true", nil
	}
	if strings.Contains(lowerPrompt, "role - project manager") || strings.Contains(lowerPrompt, "role: project manager") {
		return "agent-bridge signal PROJECT_SIGNED_OFF true", nil
	}

	// 5. Initializer Agent
	if strings.Contains(lowerPrompt, "role - initializer agent") || strings.Contains(lowerPrompt, "role: initializer agent") {
		return `
#!/bin/bash
cat << 'EOF' | agent-bridge import
{
   "project_name": "primes",
   "features": [
       {
           "id": "req-primes-implementation",
           "name": "Implement Primes Script",
           "description": "Create primes.py and primes.json",
           "priority": "1",
           "dependencies": {"depends_on_ids": []}
       }
   ]
}
EOF
`, nil
	}

	// Return a generic mock response if no heuristics matched
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

func containsTicketKeywords(s string) bool {
	// Simple heuristic for now
	return strings.Contains(s, "ticket") && (strings.Contains(s, "generate") || strings.Contains(s, "create"))
}
