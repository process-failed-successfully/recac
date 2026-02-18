package security

import (
	"regexp"
	"strings"
	"testing"
)

func TestRegexScanner_LineNumbers(t *testing.T) {
	// Create scanner with a specific test pattern instead of relying on defaults
	scanner := &RegexScanner{
		patterns: map[string]*regexp.Regexp{
			"Test Pattern": regexp.MustCompile(`TEST_MATCH`),
		},
	}
	testPattern := "TEST_MATCH"

	tests := []struct {
		name     string
		content  string
		expected []int // Expected line numbers for findings
	}{
		{
			name:     "Match on Line 1",
			content:  testPattern,
			expected: []int{1},
		},
		{
			name:     "Match on Line 2",
			content:  "\n" + testPattern,
			expected: []int{2},
		},
		{
			name:     "Match on Line 3",
			content:  "Safe\nSafe\n" + testPattern,
			expected: []int{3},
		},
		{
			name:     "Multiple matches on different lines",
			content:  testPattern + "\n" + testPattern,
			expected: []int{1, 2},
		},
		{
			name:     "Multiple matches on same line",
			content:  testPattern + " " + testPattern,
			expected: []int{1, 1},
		},
		{
			name:     "Match after many newlines",
			content:  strings.Repeat("\n", 10) + testPattern,
			expected: []int{11},
		},
		{
			name:     "Match with surrounding text",
			content:  "prefix " + testPattern + " suffix\n" + testPattern,
			expected: []int{1, 2},
		},
		{
			name:     "Match at start of file with no newline at end",
			content:  testPattern,
			expected: []int{1},
		},
		{
			name:     "Match at end of file with newline at end",
			content:  "\n" + testPattern + "\n",
			expected: []int{2},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			findings, err := scanner.Scan(tt.content)
			if err != nil {
				t.Fatalf("Scan failed: %v", err)
			}

			// Collect line numbers
			var lines []int
			for _, f := range findings {
				lines = append(lines, f.Line)
			}

			if len(lines) != len(tt.expected) {
				t.Errorf("Expected %d findings, got %d. Findings: %v", len(tt.expected), len(lines), findings)
			} else {
				for i, line := range lines {
					if line != tt.expected[i] {
						t.Errorf("Finding %d: Expected line %d, got %d", i, tt.expected[i], line)
					}
				}
			}
		})
	}
}
