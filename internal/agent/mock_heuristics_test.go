package agent

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMockAgent_Send_Heuristics(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	tests := []struct {
		name           string
		prompt         string
		expectJSON     bool
		expectKeywords []string
	}{
		{
			name:       "TPM Request",
			prompt:     "You are a Technical Program Manager. Output your plan as a JSON list.",
			expectJSON: true,
			expectKeywords: []string{
				"ID:[PRIMES]",
				"Implement Python Script",
			},
		},
		{
			name:       "Planner Request",
			prompt:     "Act as a Planner and decompose the task into a JSON array.",
			expectJSON: true,
			expectKeywords: []string{
				"ID:[PRIMES]",
			},
		},
		{
			name:       "Coding Primes",
			prompt:     "Write a python script to generate prime numbers and save to primes.json. ID:[PRIMES]",
			expectJSON: false, // Expects markdown code block
			expectKeywords: []string{
				"import json",
				"def is_prime(n):",
				"primes.json",
			},
		},
		{
			name:           "Generic Prompt",
			prompt:         "Hello world",
			expectJSON:     false,
			expectKeywords: []string{"Mock agent response"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := agent.Send(ctx, tt.prompt)
			assert.NoError(t, err)

			if tt.expectJSON {
				var js interface{}
				err := json.Unmarshal([]byte(resp), &js)
				assert.NoError(t, err, "Response should be valid JSON: %s", resp)
			}

			for _, kw := range tt.expectKeywords {
				assert.Contains(t, resp, kw)
			}
		})
	}
}
