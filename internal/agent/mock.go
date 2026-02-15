package agent

import (
	"context"
	"encoding/json"
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
	if m.forcedResponse != "" {
		return m.forcedResponse, nil
	}

	lowerPrompt := strings.ToLower(prompt)

	// completion heuristic
	if strings.Contains(lowerPrompt, "nothing to commit") {
		return "NO_OP", nil
	}

	// TPM Agent Heuristic (Jira Ticket Generation)
	if strings.Contains(prompt, "TPM Agent") || strings.Contains(prompt, "Technical Product Manager") {
		tickets := []map[string]interface{}{
			{
				"title":       "ID:[PRIMES] Implement Prime Number Generator",
				"description": "Implement a python script to generate prime numbers.\nRepo: https://github.com/example/repo",
				"type":        "Epic",
				"children": []map[string]interface{}{
					{
						"title":               "ID:[PRIMES-1] Create primes.py",
						"description":         "Create a script that writes primes to primes.json",
						"type":                "Story",
						"acceptance_criteria": []string{"primes.json is created"},
					},
				},
			},
		}
		bytes, _ := json.Marshal(tickets)
		return string(bytes), nil
	}

	// Initializer Agent (Feature extraction)
	// Prioritize this over coding to avoid coding script being returned for initializer
	if strings.Contains(prompt, "Initializer Agent") || strings.Contains(prompt, "extract features") {
		return `["PRIMES-1"]`, nil
	}

	// Coding Agent - Primes Scenario
	if strings.Contains(lowerPrompt, "prime") || strings.Contains(lowerPrompt, "primes.json") || strings.Contains(lowerPrompt, "id:[primes]") {
		script := "```python\nimport json\n\ndef is_prime(n):\n    if n < 2:\n        return False\n    for i in range(2, int(n**0.5) + 1):\n        if n % i == 0:\n            return False\n    return True\n\nprimes = [x for x in range(2, 100) if is_prime(x)]\n\nwith open('primes.json', 'w') as f:\n    json.dump({'primes': primes}, f)\nprint('Done')\n```"
		// If prompt asks to run it or something, we assume agent bridge runs it.
		// If prompt asks for "next step", we return script.
		return script, nil
	}

	// QA Agent
	if strings.Contains(prompt, "QA Agent") || strings.Contains(lowerPrompt, "qa_passed") {
		return "QA_PASSED", nil
	}

	// Manager Agent
	if strings.Contains(prompt, "Manager Agent") {
		return "PROJECT_SIGNED_OFF", nil
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
