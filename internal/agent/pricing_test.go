package agent

import (
	"testing"
)

func TestCalculateCost(t *testing.T) {
	testCases := []struct {
		name     string
		model    string
		usage    TokenUsage
		expected float64
	}{
		{
			name:  "Gemini Pro",
			model: "gemini-pro",
			usage: TokenUsage{
				TotalPromptTokens:   1000000,
				TotalResponseTokens: 1000000,
			},
			expected: 0.50 + 1.50, // 2.0
		},
		{
			name:  "GPT-4o",
			model: "gpt-4o",
			usage: TokenUsage{
				TotalPromptTokens:   500000,
				TotalResponseTokens: 200000,
			},
			expected: (0.5 * 5.00) + (0.2 * 15.00), // 2.5 + 3.0 = 5.5
		},
		{
			name:  "Unknown Model",
			model: "unknown-model",
			usage: TokenUsage{
				TotalPromptTokens:   1000,
				TotalResponseTokens: 1000,
			},
			// TotalTokens = 2000. 2000/1M = 0.002
			expected: 0.002,
		},
		{
			name:  "Zero Usage",
			model: "gpt-4o",
			usage: TokenUsage{
				TotalPromptTokens:   0,
				TotalResponseTokens: 0,
			},
			expected: 0.0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.usage.TotalTokens = tc.usage.TotalPromptTokens + tc.usage.TotalResponseTokens
			cost := CalculateCost(tc.model, tc.usage)
			if cost != tc.expected {
				t.Errorf("expected cost %.4f, got %.4f", tc.expected, cost)
			}
		})
	}
}
