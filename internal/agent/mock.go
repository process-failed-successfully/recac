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
func (m *MockAgent) Send(ctx context.Context, prompt string) (string, error) {
	if m.forcedResponse != "" {
		return m.forcedResponse, nil
	}

	lowerPrompt := strings.ToLower(prompt)

	// Heuristic 1: TPM/Architect - Generate Ticket Plan
	// Triggers when asking for a plan (JSON) and mentions TPM roles
	if (strings.Contains(lowerPrompt, "technical program manager") || strings.Contains(lowerPrompt, "architect") || strings.Contains(lowerPrompt, "planner")) && strings.Contains(lowerPrompt, "json") {
		return m.generateMockTicketPlan()
	}

	// Heuristic 2: Coding - Primes Scenario
	// Triggers when asking for primes
	if strings.Contains(lowerPrompt, "primes") || strings.Contains(lowerPrompt, "prime number") {
		return m.generatePrimesScript()
	}

	// Default Mock Response
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

func (m *MockAgent) generateMockTicketPlan() (string, error) {
	// Structure matching ticketNode in cmd/recac/jira.go
	type ticketNode struct {
		Title              string       `json:"title"`
		Description        string       `json:"description"`
		Type               string       `json:"type"`
		BlockedBy          []string     `json:"blocked_by"`
		AcceptanceCriteria []string     `json:"acceptance_criteria"`
		Children           []ticketNode `json:"children"`
	}

	// Note: We use ID:[PRIMES] for the Story to ensure the E2E runner can map it correctly.
	plan := []ticketNode{
		{
			Title:       "ID:[SYSTEM] Prime Number System",
			Description: "Implement a system to generate and store prime numbers.\n\nRepo: https://github.com/process-failed-successfully/recac-jira-e2e",
			Type:        "Epic",
			Children: []ticketNode{
				{
					Title:       "ID:[PRIMES] Implement Python Script",
					Description: "Create a python script 'primes.py' that generates prime numbers up to 100 and saves them to 'primes.json'.",
					Type:        "Story",
					AcceptanceCriteria: []string{
						"Script generates correct primes",
						"Output is valid JSON",
					},
				},
			},
		},
	}

	bytes, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		return "", err
	}

	// Wrap in markdown code block as Agents often do
	return fmt.Sprintf("Here is the ticket plan:\n```json\n%s\n```", string(bytes)), nil
}

func (m *MockAgent) generatePrimesScript() (string, error) {
	// Returns a bash script that sets up git, creates the python script, runs it, and commits it.
	// This satisfies the requirement to "Implement Python Script" and ensures artifacts exist.

	script := `#!/bin/bash
set -e

# Configure Git
git config user.email "agent@recac.io"
git config user.name "Recac Agent"

# Create Python Script
cat <<EOF > primes.py
import json

def is_prime(n):
    if n <= 1:
        return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0:
            return False
    return True

primes = [x for x in range(101) if is_prime(x)]
result = {"primes": primes}

with open("primes.json", "w") as f:
    json.dump(result, f)

print(f"Generated {len(primes)} primes")
EOF

# Run it
python3 primes.py

# Commit
git add primes.py primes.json
git commit -m "Implement primes generation script"
`

	return fmt.Sprintf("I will implement the prime number generator.\n\n```bash\n%s\n```", script), nil
}

// truncateString truncates a string to a maximum length
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}
