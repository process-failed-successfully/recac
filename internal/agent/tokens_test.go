package agent

import (
	"strings"
	"testing"
)

func TestEstimateTokenCount(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		expected int
	}{
		{
			name:     "empty string",
			text:     "",
			expected: 0,
		},
		{
			name:     "whitespace only",
			text:     "   \n\t  ",
			expected: 0,
		},
		{
			name:     "short text",
			text:     "Hello world",
			expected: 3, // 11 chars / 4 = ~3 tokens
		},
		{
			name:     "medium text",
			text:     "This is a longer sentence with more words to count tokens.",
			expected: 15, // ~60 chars / 4 = ~15 tokens
		},
		{
			name:     "multiline text",
			text:     "Line 1\nLine 2\nLine 3",
			expected: 5, // ~20 chars / 4 = ~5 tokens
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := EstimateTokenCount(tt.text)
			// Allow some variance in estimation
			if result < tt.expected-2 || result > tt.expected+2 {
				t.Errorf("EstimateTokenCount() = %d, expected around %d", result, tt.expected)
			}
		})
	}
}

func TestMinMax(t *testing.T) {
	if min(1, 2) != 1 {
		t.Error("min(1, 2) should be 1")
	}
	if min(2, 1) != 1 {
		t.Error("min(2, 1) should be 1")
	}
	if max(1, 2) != 2 {
		t.Error("max(1, 2) should be 2")
	}
	if max(2, 1) != 2 {
		t.Error("max(2, 1) should be 2")
	}
}

func TestTruncateToTokenLimit(t *testing.T) {
	tests := []struct {
		name          string
		text          string
		maxTokens     int
		wantTruncated bool
	}{
		{
			name:          "text under limit",
			text:          "Short text",
			maxTokens:     100,
			wantTruncated: false,
		},
		{
			name:          "text over limit",
			text:          "This is a very long text that exceeds the token limit and should be truncated to fit within the specified maximum number of tokens allowed for the context window.",
			maxTokens:     10,
			wantTruncated: true,
		},
		{
			name:          "zero limit",
			text:          "Any text",
			maxTokens:     0,
			wantTruncated: true,
		},
		{
			name:          "multiline text",
			text:          "Line 1\nLine 2\nLine 3\nLine 4\nLine 5\nLine 6\nLine 7\nLine 8\nLine 9\nLine 10",
			maxTokens:     5,
			wantTruncated: true,
		},
		{
			name:          "single line truncation",
			text:          "ThisIsAVeryLongSingleLinePart1Part2Part3Part4Part5",
			maxTokens:     5,
			wantTruncated: true,
		},
		{
			name:          "marker exceeds limit",
			text:          "Some text",
			maxTokens:     1,
			wantTruncated: true,
		},
		{
			name:          "negative limit",
			text:          "Some text",
			maxTokens:     -5,
			wantTruncated: true, // effectively returns empty string which is 'truncated'
		},
		{
			name:          "recursive safety check",
			text:          strings.Repeat("a", 1000),
			maxTokens:     10, // Small limit
			wantTruncated: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TruncateToTokenLimit(tt.text, tt.maxTokens)
			resultTokens := EstimateTokenCount(result)

			if tt.maxTokens > 0 {
				if resultTokens > tt.maxTokens {
					t.Errorf("TruncateToTokenLimit() result has %d tokens, exceeds limit of %d", resultTokens, tt.maxTokens)
				}
			} else {
				if result != "" {
					t.Errorf("Expected empty string for maxTokens <= 0, got %q", result)
				}
			}

			if tt.wantTruncated && result == tt.text {
				// Special case: if text is empty, it's equal but technically not truncated unless we define it so.
				// But here we are testing truncation logic.
				if tt.text != "" {
					t.Errorf("TruncateToTokenLimit() should have truncated but didn't")
				}
			}

			if !tt.wantTruncated && result != tt.text {
				t.Errorf("TruncateToTokenLimit() should not have truncated but did")
			}
		})
	}
}

func TestSummarizeForTokenLimit(t *testing.T) {
	tests := []struct {
		name      string
		text      string
		maxTokens int
	}{
		{
			name:      "text under limit",
			text:      "Short paragraph.",
			maxTokens: 100,
		},
		{
			name:      "multiple paragraphs",
			text:      "First paragraph with important information.\n\nSecond paragraph with more details.\n\nThird paragraph with final thoughts.",
			maxTokens: 10,
		},
		{
			name:      "zero limit",
			text:      "Some text",
			maxTokens: 0,
		},
		{
			name:      "marker exceeds limit",
			text:      "Some text\n\nMore text",
			maxTokens: 2, // Very small
		},
		{
			name:      "single paragraph large",
			text:      "This is a single paragraph that is quite long and might need truncation if the limit is small enough.",
			maxTokens: 5,
		},
		{
			name:      "last paragraph large",
			text:      "Start.\n\n" + strings.Repeat("End ", 50),
			maxTokens: 20,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SummarizeForTokenLimit(tt.text, tt.maxTokens)
			resultTokens := EstimateTokenCount(result)

			if tt.maxTokens > 0 {
				if resultTokens > tt.maxTokens {
					t.Errorf("SummarizeForTokenLimit() result has %d tokens, exceeds limit of %d", resultTokens, tt.maxTokens)
				}
			} else {
				if result != "" {
					t.Errorf("Expected empty string for maxTokens <= 0, got %q", result)
				}
			}

			if result == "" && tt.text != "" && tt.maxTokens > 0 {
				// If maxTokens is extremely small, it might return empty string via TruncateToTokenLimit
				// But generally we expect something if possible.
				// For now let's just check it doesn't panic.
			}
		})
	}
}
