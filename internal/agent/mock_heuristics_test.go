package agent

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMockAgent_Heuristics(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	tests := []struct {
		name           string
		prompt         string
		expectJSON     bool
		expectKeywords []string
	}{
		{
			name:   "TPM Heuristic",
			prompt: "You are a Technical Program Manager. Please generate a ticket list in JSON format.",
			expectJSON: true,
			expectKeywords: []string{
				"ID:[PRIMES]",
				"Implement Python Script",
			},
		},
		{
			name:   "Architect Heuristic",
			prompt: "Please break down the application specification into a feature list.",
			expectJSON: true,
			expectKeywords: []string{
				"project_name",
				"Mock Project",
			},
		},
		{
			name:   "Coding Heuristic - Primes",
			prompt: "Write a script to generate prime numbers and save to primes.json.",
			expectJSON: false, // It returns markdown code block
			expectKeywords: []string{
				"def generate_primes",
				"primes.json",
			},
		},
		{
			name:   "Manager Heuristic",
			prompt: "I am the Lead Software Architect. Please review.",
			expectJSON: false,
			expectKeywords: []string{
				"Approving the plan",
			},
		},
		{
			name:   "Completion Heuristic",
			prompt: "git status shows nothing to commit.",
			expectJSON: false,
			expectKeywords: []string{
				"Done.",
			},
		},
		{
			name:   "Fallback",
			prompt: "Hello world",
			expectJSON: false,
			expectKeywords: []string{
				"Mock agent response",
				"I received your prompt",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := agent.Send(ctx, tt.prompt)
			assert.NoError(t, err)

			for _, keyword := range tt.expectKeywords {
				assert.Contains(t, resp, keyword, "Response should contain keyword: %s", keyword)
			}

			if tt.expectJSON {
				var js interface{}
				err := json.Unmarshal([]byte(resp), &js)
				assert.NoError(t, err, "Response should be valid JSON. Got: %s", resp)
			}
		})
	}
}
