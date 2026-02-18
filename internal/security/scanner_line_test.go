package security

import (
	"regexp"
	"testing"
)

func TestScanner_LineNumbers(t *testing.T) {
	// Create a scanner with a custom pattern for testing
	scanner := &RegexScanner{
		patterns: map[string]*regexp.Regexp{
			"Test Pattern": regexp.MustCompile(`TEST`),
		},
	}

	tests := []struct {
		name     string
		content  string
		expected []int // Expected line numbers for each match
	}{
		{
			name:     "Single line match",
			content:  "This is a TEST",
			expected: []int{1},
		},
		{
			name:     "Match on second line",
			content:  "Line 1\nLine 2 TEST",
			expected: []int{2},
		},
		{
			name:     "Match at beginning of line",
			content:  "Line 1\nTEST on line 2",
			expected: []int{2},
		},
		{
			name:     "Match at end of line",
			content:  "Line 1 TEST\nLine 2",
			expected: []int{1},
		},
		{
			name:     "Match spanning multiple lines (not applicable for this regex)",
			content:  "TEST\nTEST",
			expected: []int{1, 2},
		},
		{
			name:     "Multiple matches on same line",
			content:  "TEST TEST",
			expected: []int{1, 1},
		},
		{
			name:     "Matches with multiple newlines",
			content:  "\n\nTEST\n\n",
			expected: []int{3},
		},
		{
			name:     "Empty string",
			content:  "",
			expected: []int{},
		},
		{
			name:     "Match at very beginning",
			content:  "TEST",
			expected: []int{1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			findings, err := scanner.Scan(tt.content)
			if err != nil {
				t.Fatalf("Scan failed: %v", err)
			}

			if len(findings) != len(tt.expected) {
				t.Errorf("Expected %d findings, got %d", len(tt.expected), len(findings))
			}

			for i, finding := range findings {
				if i < len(tt.expected) && finding.Line != tt.expected[i] {
					t.Errorf("Finding %d: expected line %d, got %d", i, tt.expected[i], finding.Line)
				}
			}
		})
	}
}
