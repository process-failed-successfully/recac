package agent

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCalculateCost(t *testing.T) {
	tests := []struct {
		name     string
		model    string
		usage    TokenUsage
		expected float64
	}{
		{
			name:  "Gemini Pro Calculation",
			model: "gemini-pro",
			usage: TokenUsage{
				TotalPromptTokens:   1_000_000,
				TotalResponseTokens: 1_000_000,
			},
			expected: 0.50 + 1.50, // 2.00
		},
		{
			name:  "GPT-4o Calculation",
			model: "gpt-4o",
			usage: TokenUsage{
				TotalPromptTokens:   500_000,
				TotalResponseTokens: 200_000,
			},
			expected: (0.5 * 5.00) + (0.2 * 15.00), // 2.5 + 3.0 = 5.5
		},
		{
			name:  "Unknown Model Fallback",
			model: "unknown-model",
			usage: TokenUsage{
				TotalTokens: 2_000_000,
			},
			expected: 2.0, // 1.0 per million? No, fallback logic: float64(usage.TotalTokens) / 1_000_000.0
		},
		{
			name:  "Zero Usage",
			model: "gpt-4o",
			usage: TokenUsage{},
			expected: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cost := CalculateCost(tt.model, tt.usage)
			assert.InDelta(t, tt.expected, cost, 0.0001)
		})
	}
}
