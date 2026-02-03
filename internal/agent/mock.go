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

	// 1. Handle TPM / Ticket Generation Prompt (JSON output)
	if strings.Contains(prompt, "Technical Program Manager") || strings.Contains(prompt, "generate-from-spec") {
		// Extract ID if present, e.g., ID:[PRIMES]
		reID := regexp.MustCompile(`ID:\[(.*?)\]`)
		matches := reID.FindStringSubmatch(prompt)
		id := "TASK-1"
		title := "Mock Task"
		if len(matches) > 1 {
			id = matches[1]
			title = fmt.Sprintf("ID:[%s] Mock Task", id)
		}

		// Return JSON for recac CLI
		// We extract extracted ID to ensure it maps correctly in the runner
		return fmt.Sprintf(`[
  {
    "title": "%s",
    "description": "Mock description for %s. Repo: https://github.com/process-failed-successfully/recac-jira-e2e",
    "type": "Task",
    "acceptance_criteria": ["Create primes.py"],
    "children": []
  }
]`, title, id), nil
	}

	// 2. Handle Initializer (feature_list.json) - if requested
	if strings.Contains(prompt, "Initialize") && strings.Contains(prompt, "feature_list.json") {
		return "Here is the initialization script:\n\n```bash\ncat << 'EOF' > feature_list.json\n{\n  \"project_name\": \"mock-project\",\n  \"features\": [\n    {\"id\": \"req-primes-py-exists\", \"description\": \"primes.py exists\", \"status\": \"todo\", \"type\": \"file_exists\", \"target\": \"primes.py\"}\n  ]\n}\nEOF\n```\n", nil
	}

	// 3. Handle Coding Agent (primes.py)
	if strings.Contains(prompt, "primes.py") {
		return "Here is the script to generate primes:\n\n```bash\n#!/bin/bash\nset -e\n\n# Configure git if needed\ngit config user.email \"mock@agent.com\" || true\ngit config user.name \"Mock Agent\" || true\n\n# Create primes.py\ncat << 'EOF' > primes.py\nimport json\n\ndef is_prime(n):\n    if n < 2: return False\n    for i in range(2, int(n**0.5) + 1):\n        if n % i == 0:\n            return False\n    return True\n\nprimes = [i for i in range(10000) if is_prime(i)]\n\nwith open('primes.json', 'w') as f:\n    json.dump({\"primes\": primes}, f)\nEOF\n\n# Run it to generate output\npython3 primes.py\n\n# Add and commit\ngit add primes.py primes.json\ngit commit -m \"Add primes.py and output\" || echo \"Nothing to commit\"\n\n# Signal completion if bridge is available\nif command -v agent-bridge &> /dev/null; then\n    agent-bridge update --status done --feature req-primes-py-exists || true\nfi\n```\n", nil
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
