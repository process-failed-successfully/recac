package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
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
// It returns a mock response that acknowledges the prompt
func (m *MockAgent) Send(ctx context.Context, prompt string) (string, error) {
	if m.forcedResponse != "" {
		return m.forcedResponse, nil
	}

	// Heuristic Role Detection

	// 1. Technical Program Manager (TPM)
	// E2E smoke tests rely on this specific phrase to detect TPM role
	if strings.Contains(prompt, "You are an expert Technical Program Manager (TPM)") {
		fmt.Println("[MockAgent] Detected TPM Role")

		// Attempt to extract Ticket ID for consistency
		ticketID := "TASK-1"
		re := regexp.MustCompile(`ID:\[?([\w-]+)\]?`)
		matches := re.FindStringSubmatch(prompt)
		if len(matches) > 1 {
			ticketID = matches[1]
		}

		// Also check env var which is authoritative in some flows
		if envID := os.Getenv("RECAC_PROJECT_ID"); envID != "" {
			ticketID = envID
		}

		// Return a valid JSON plan
		type ticketNode struct {
			Title       string `json:"title"`
			Description string `json:"description"`
			Type        string `json:"type"`
			Status      string `json:"status"`
			Children    []ticketNode `json:"children,omitempty"`
		}

		tickets := []ticketNode{
			{
				Title:       fmt.Sprintf("ID:[%s] Implement Primes", ticketID),
				Description: "Implement a python script to calculate primes",
				Type:        "Task",
				Status:      "todo",
			},
		}

		data, _ := json.MarshalIndent(tickets, "", "  ")
		return string(data), nil
	}

	// 2. Initializer Agent
	if strings.Contains(prompt, "## YOUR ROLE - INITIALIZER AGENT") {
		fmt.Println("[MockAgent] Detected Initializer Role")
		// Return a bash script that imports features
		return `#!/bin/bash
cat << 'EOF' | agent-bridge import
{
  "features": [
    {
      "id": "PRIMES-1",
      "description": "Implement primes.py",
      "status": "todo",
      "category": "Core"
    }
  ]
}
EOF
`, nil
	}

	// 3. QA Agent
	if strings.Contains(prompt, "## YOUR ROLE - QA AGENT") {
		fmt.Println("[MockAgent] Detected QA Role")
		return "QA_PASSED", nil
	}

	// 4. Project Manager
	if strings.Contains(prompt, "## YOUR ROLE - PROJECT MANAGER") {
		fmt.Println("[MockAgent] Detected Manager Role")
		// Simulate occasional rejection or approval? For smoke test, we want success.
		return "PROJECT_SIGNED_OFF", nil
	}

	// 5. Coding Agent
	// Check for coding keywords or explicit header
	if strings.Contains(prompt, "## YOUR ROLE - CODING AGENT") ||
	   strings.Contains(prompt, "[PRIMES]") ||
	   strings.Contains(prompt, "primes.py") ||
	   strings.Contains(prompt, "Prime Number Script") {

		fmt.Println("[MockAgent] Detected Coding Role")

		// Basic Python implementation
		return `#!/bin/bash
# Implement primes.py
cat << 'EOF' > primes.py
def is_prime(n):
    if n <= 1: return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0: return False
    return True

if __name__ == "__main__":
    import sys
    n = int(sys.argv[1]) if len(sys.argv) > 1 else 10
    print([x for x in range(n) if is_prime(x)])
EOF

# Signal completion via agent-bridge (simulated if not available)
agent-bridge feature update PRIMES-1 --status completed || true
echo "Implementation complete."
`, nil
	}

	// Default / Fallback Response
	response := fmt.Sprintf("%s:\n\nI received your prompt (%d characters). In mock mode, I would process this request and provide a response. The actual implementation would call the AI provider API here.\n\nPrompt preview: %s...",
		m.responsePrefix, len(prompt), truncateString(prompt, 100))
	return response, nil
}

// SendStream implements the Agent interface
func (m *MockAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	// Simulate latency for realism
	time.Sleep(10 * time.Millisecond)

	resp, err := m.Send(ctx, prompt)
	if err == nil && onChunk != nil {
		// Simulate chunking
		chunkSize := 10
		runes := []rune(resp)
		for i := 0; i < len(runes); i += chunkSize {
			end := i + chunkSize
			if end > len(runes) {
				end = len(runes)
			}
			onChunk(string(runes[i:end]))
			time.Sleep(1 * time.Millisecond)
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
