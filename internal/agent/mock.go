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

	// Heuristic: Check if this is a Planning Phase prompt (TPM Agent)
	// We check for keywords that appear in the TPM prompt or the expected output format.
	if contains(prompt, "Technical Program Manager") || contains(prompt, "Output purely JSON") {
		// Return a predefined JSON response compatible with the CLI's expectations
		// Extract repo URL from prompt if possible to make it more realistic
		repoURL := "https://github.com/example/repo"
		if matches := regexp.MustCompile(`(?i)Repo: (https?://\S+)`).FindStringSubmatch(prompt); len(matches) > 1 {
			repoURL = matches[1]
		}

		if contains(prompt, "ID:[PRIMES]") {
			return fmt.Sprintf(`[
  {
    "title": "ID:[PRIMES] Implement Primes Service",
    "description": "Implement a service that calculates prime numbers. Repo: %s",
    "type": "Epic",
    "acceptance_criteria": [
      "Service calculates primes correctly",
      "API returns JSON"
    ],
    "children": [
      {
        "title": "ID:[PRIMES-API] Implement API",
        "description": "Implement the HTTP API for the primes service. Repo: %s",
        "type": "Story",
        "acceptance_criteria": [
          "GET /primes/{n} returns prime check",
          "Returns 200 OK"
        ]
      }
    ]
  }
]`, repoURL, repoURL), nil
		}
		// Default JSON for other TPM prompts
		return `[]`, nil
	}

	// Heuristic: Check if this is a Coding Agent prompt
	if contains(prompt, "YOUR ROLE - CODING AGENT") || contains(prompt, "Coding Agent") {
		// Extract Feature ID to mark as done
		// Prompt format: "**Feature ID**: {task_id}"
		featureID := "unknown"
		if matches := regexp.MustCompile(`\*\*Feature ID\*\*: ([\w-]+)`).FindStringSubmatch(prompt); len(matches) > 1 {
			featureID = matches[1]
		}

		if featureID == "NONE_ALL_COMPLETE" {
			// All features complete, signal completion
			return "```bash\nagent-bridge signal COMPLETED true\necho \"Signal COMPLETED set\"\n```", nil
		}

		return fmt.Sprintf("```bash\n#!/bin/bash\n# Mock implementation for %s\necho \"Implementing feature %s...\"\nagent-bridge feature set %s --status done --passes true || echo \"Failed to set status for %s\"\necho \"Success: Mock command executed\"\n```", featureID, featureID, featureID, featureID), nil
	}

	// Heuristic: Check if this is a QA Agent prompt
	if contains(prompt, "YOUR ROLE - QA AGENT") {
		return "```bash\n#!/bin/bash\necho \"Running QA checks...\"\nagent-bridge signal QA_PASSED true\necho \"QA Passed\"\n```", nil
	}

	// Heuristic: Check if this is a Project Manager prompt
	if contains(prompt, "YOUR ROLE - PROJECT MANAGER") {
		return "```bash\n#!/bin/bash\necho \"Manager Review...\"\nagent-bridge signal PROJECT_SIGNED_OFF true\necho \"Project Signed Off\"\n```", nil
	}

	// Return a mock response that shows the agent received the prompt
	// This allows the session to run without requiring real API keys
	response := fmt.Sprintf("%s:\n\nI received your prompt (%d characters). In mock mode, I would process this request and provide a response. The actual implementation would call the AI provider API here.\n\nPrompt preview: %s...",
		m.responsePrefix, len(prompt), truncateString(prompt, 100))
	return response, nil
}

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
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
