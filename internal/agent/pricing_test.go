package agent

import (
	"testing"
)

func TestCalculateCost(t *testing.T) {
	tests := []struct {
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
			expected: 0.50 + 1.50, // 2.00
		},
		{
			name:  "GPT-4o",
			model: "gpt-4o",
			usage: TokenUsage{
				TotalPromptTokens:   500000, // 0.5 million
				TotalResponseTokens: 200000, // 0.2 million
			},
			expected: (0.5 * 5.00) + (0.2 * 15.00), // 2.5 + 3.0 = 5.5
		},
		{
			name:  "Claude 3 Opus",
			model: "claude-3-opus-20240229",
			usage: TokenUsage{
				TotalPromptTokens:   100000, // 0.1 million
				TotalResponseTokens: 100000, // 0.1 million
			},
			expected: (0.1 * 15.00) + (0.1 * 75.00), // 1.5 + 7.5 = 9.0
		},
		{
			name:  "Unknown Model",
			model: "unknown-model",
			usage: TokenUsage{
				TotalTokens: 1000000,
			},
			expected: 1.00, // Fallback: total / 1,000,000
		},
		{
			name:  "Zero Usage",
			model: "gpt-4o",
			usage: TokenUsage{
				TotalPromptTokens:   0,
				TotalResponseTokens: 0,
			},
			expected: 0.00,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cost := CalculateCost(tt.model, tt.usage)

			// Use a small epsilon for float comparison
			epsilon := 0.000001
			if cost < tt.expected-epsilon || cost > tt.expected+epsilon {
				t.Errorf("CalculateCost() = %f, expected %f", cost, tt.expected)
			}
		})
	}
}
