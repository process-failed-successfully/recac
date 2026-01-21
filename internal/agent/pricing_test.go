package agent

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCalculateCost(t *testing.T) {
	tests := []struct {
		name         string
		model        string
		usage        TokenUsage
		expectedCost float64
	}{
		{
			name:  "Gemini 1.5 Pro",
			model: "gemini-1.5-pro-latest",
			usage: TokenUsage{
				TotalPromptTokens:   1_000_000,
				TotalResponseTokens: 1_000_000,
			},
			expectedCost: 7.00 + 21.00,
		},
		{
			name:  "GPT-4o",
			model: "gpt-4o",
			usage: TokenUsage{
				TotalPromptTokens:   500_000,
				TotalResponseTokens: 200_000,
			},
			expectedCost: (0.5 * 5.00) + (0.2 * 15.00),
		},
		{
			name:  "Unknown Model",
			model: "unknown-model",
			usage: TokenUsage{
				TotalTokens: 2_000_000,
			},
			expectedCost: 2.0, // 2_000_000 / 1_000_000
		},
		{
			name:  "Zero Usage",
			model: "gpt-3.5-turbo",
			usage: TokenUsage{
				TotalPromptTokens:   0,
				TotalResponseTokens: 0,
			},
			expectedCost: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cost := CalculateCost(tt.model, tt.usage)
			assert.InDelta(t, tt.expectedCost, cost, 0.0001)
		})
	}
}
