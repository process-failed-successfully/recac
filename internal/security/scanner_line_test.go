package security

import (
	"regexp"
	"strings"
	"testing"
)

func TestScannerLineNumbers(t *testing.T) {
	s := NewRegexScanner()
	// Use a controlled pattern for testing line numbers
	s.patterns = map[string]*regexp.Regexp{
		"TEST": regexp.MustCompile(`TEST`),
	}

	tests := []struct {
		name     string
		content  string
		expected int
	}{
		{"Start of line 1", "TEST", 1},
		{"Start of line 2", "\nTEST", 2},
		{"End of line 1", "TEST\n", 1},
		{"Middle of line 2", "\nABC TEST", 2},
		{"After multiple lines", "\n\n\nTEST", 4},
		{"On newline boundary", "A\nTEST", 2},
		{"Large input no newlines", strings.Repeat("a", 1000) + "TEST", 1},
		{"CR only", "TEST\r", 1}, // CR doesn't count as newline here
		{"CR before TEST", "\rTEST", 1},
		{"CRLF", "\r\nTEST", 2},
		{"Mixed", "\nTEST\r\n", 2},
		{"Mixed 2", "A\nB\r\nTEST", 3},
		{"Many newlines before match", strings.Repeat("\n", 100) + "TEST", 101},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			findings, err := s.Scan(tt.content)
			if err != nil {
				t.Fatalf("Scan failed: %v", err)
			}
			if len(findings) != 1 {
				t.Fatalf("Expected 1 finding, got %d", len(findings))
			}
			if findings[0].Line != tt.expected {
				t.Errorf("Expected line %d, got %d for content %q", tt.expected, findings[0].Line, tt.content)
			}
		})
	}
}

func TestScannerEmptyContent(t *testing.T) {
	s := NewRegexScanner()
	findings, err := s.Scan("")
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	if len(findings) != 0 {
		t.Errorf("Expected 0 findings, got %d", len(findings))
	}
}

func TestScannerNoNewlines(t *testing.T) {
	s := NewRegexScanner()
	s.patterns = map[string]*regexp.Regexp{
		"TEST": regexp.MustCompile(`TEST`),
	}
	findings, err := s.Scan("ABCDEF")
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	if len(findings) != 0 {
		t.Errorf("Expected 0 findings, got %d", len(findings))
	}
}
