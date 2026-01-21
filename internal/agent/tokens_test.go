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
			name:          "single line unicode",
			text:          "Hello 世界 World 🌍 This is a test for unicode characters.",
			maxTokens:     5,
			wantTruncated: true,
		},
		{
			name:          "recursive safety",
			text:          strings.Repeat("a", 1000),
			maxTokens:     10,
			wantTruncated: true,
		},
		{
			name:          "exact fit start cut",
			text:          "Line 1\nLine 2\nLine 3\nLine 4\nLine 5",
			maxTokens:     100, // Large enough
			wantTruncated: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TruncateToTokenLimit(tt.text, tt.maxTokens)
			resultTokens := EstimateTokenCount(result)

			if tt.maxTokens > 0 && resultTokens > tt.maxTokens {
				t.Errorf("TruncateToTokenLimit() result has %d tokens, exceeds limit of %d", resultTokens, tt.maxTokens)
			}

			if tt.wantTruncated && result == tt.text {
				t.Errorf("TruncateToTokenLimit() should have truncated but didn't")
			}

			if !tt.wantTruncated && result != tt.text {
				t.Errorf("TruncateToTokenLimit() should not have truncated but did")
			}
		})
	}
}

func TestSummarizeForTokenLimit(t *testing.T) {
	longPara := strings.Repeat("This is a long paragraph. ", 50) // ~1300 chars ~325 tokens

	tests := []struct {
		name      string
		text      string
		maxTokens int
		wantLen   int // 0 means just check maxTokens
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
			name:      "many paragraphs",
			text:      "P1\n\nP2\n\nP3\n\nP4\n\nP5",
			maxTokens: 10, // Very tight
		},
		{
			name:      "long paragraphs truncation",
			text:      longPara + "\n\n" + longPara + "\n\n" + longPara,
			maxTokens: 50,
		},
		{
			name:      "marker exceeds limit",
			text:      "P1\n\nP2",
			maxTokens: 2, // Too small for marker -> fallback to truncate
		},
		{
			name:      "single paragraph",
			text:      longPara,
			maxTokens: 50,
		},
		{
			name:      "empty text",
			text:      "",
			maxTokens: 10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SummarizeForTokenLimit(tt.text, tt.maxTokens)
			resultTokens := EstimateTokenCount(result)

			if resultTokens > tt.maxTokens {
				t.Errorf("SummarizeForTokenLimit() result has %d tokens, exceeds limit of %d", resultTokens, tt.maxTokens)
			}

			if tt.text != "" && result == "" && tt.maxTokens > 5 {
				t.Errorf("SummarizeForTokenLimit() returned empty string for non-empty input")
			}
		})
	}
}

func TestEstimateTokenCount_EdgeCases(t *testing.T) {
	if EstimateTokenCount("") != 0 {
		t.Error("EstimateTokenCount(empty) != 0")
	}
}

func TestTruncateToTokenLimit_EdgeCases(t *testing.T) {
	// Start overlap end
	text := "Hello\nWorld"
	res := TruncateToTokenLimit(text, 100)
	if res != text {
		t.Error("Should not truncate if fits")
	}
}
