package agent

import (
	"context"
	"fmt"
	"regexp"
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

	// === Heuristics for Smoke Tests ===

	// 1. Initializer Agent
	if strings.Contains(prompt, "INITIALIZER AGENT") {
		return "```bash\n# Initializer Agent Setup\necho \"Setting up environment...\"\n# Create feature list to proceed\necho '{\"project_name\":\"test\",\"features\":[{\"id\":\"1\",\"description\":\"Implement features\",\"status\":\"pending\"}]}' > feature_list.json\n```", nil
	}

	// 2. Technical Program Manager (TPM) - Generates Jira Tickets (JSON)
	// Trigger: "Technical Program Manager" or "TPM"
	if strings.Contains(prompt, "Technical Program Manager") || strings.Contains(prompt, "TPM") {
		// Extract repo URL if possible for better realism (optional)
		repoURL := "https://github.com/example/repo"
		reRepo := regexp.MustCompile(`(?i)Repo: (https?://\S+)`)
		if match := reRepo.FindStringSubmatch(prompt); len(match) > 1 {
			repoURL = match[1]
		}

		// Return a valid JSON list of tickets
		// The orchestrator expects a list of objects with title, description, type, etc.
		// Note: "type" should be "Task" to pass filters.
		return fmt.Sprintf(`[
  {
    "title": "ID:[PRIMES] Implement Prime Number Generator",
    "description": "Create a python script that prints prime numbers.\nRepo: %s",
    "type": "Task",
    "labels": ["recac-agent"]
  }
]`, repoURL), nil
	}

	// 3. Coding Agent / Developer
	// Trigger: "CODING AGENT" or "DEVELOPER" or specific task IDs
	if strings.Contains(prompt, "CODING AGENT") || strings.Contains(prompt, "DEVELOPER") || strings.Contains(prompt, "[PRIMES]") {
		// Heuristic: If prompt asks for prime numbers (detected via [PRIMES] or feature ID), return Python code.
		if strings.Contains(prompt, "prime") || strings.Contains(prompt, "python") {
			return "I will implement the prime number generator.\n\n```bash\n# Configure git\ngit config user.email \"agent@recac.com\"\ngit config user.name \"Recac Agent\"\n```\n\n```python\n# primes.py\nimport json\n\ndef is_prime(n):\n    if n <= 1: return False\n    for i in range(2, int(n**0.5) + 1):\n        if n % i == 0: return False\n    return True\n\nprimes = [x for x in range(10000) if is_prime(x)]\nprint(json.dumps({\"primes\": primes}))\n```\n\n```bash\ngit add primes.py\ngit commit -m \"Implement prime number generator\" || echo \"Nothing to commit\"\ngit push\n```\n", nil
		}
	}

	// 4. QA Agent
	if strings.Contains(prompt, "QA AGENT") {
		return "I have verified the changes.\n\n```bash\nagent-bridge signal --privileged QA_PASSED true\n```\n", nil
	}

	// 5. Project Manager (Sign-off)
	if strings.Contains(prompt, "PROJECT MANAGER") {
		return "The project is complete.\n\n```bash\nagent-bridge signal --privileged PROJECT_SIGNED_OFF true\n```\n", nil
	}

	// 6. Loop Breaker / Generic
	if strings.Contains(prompt, "nothing to commit") || strings.Contains(prompt, "working tree clean") {
		return "It seems we are done.\n\n```bash\nagent-bridge signal --privileged QA_PASSED true\nagent-bridge signal --privileged PROJECT_SIGNED_OFF true\n```\n", nil
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
