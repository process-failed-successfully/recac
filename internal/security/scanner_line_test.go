package security

import (
	"testing"
)

func TestRegexScanner_LineNumbers(t *testing.T) {
	scanner := NewRegexScanner()

	tests := []struct {
		name        string
		content     string
		expectLines map[string]int // Finding Type -> Line Number
	}{
		{
			name:    "Single line match",
			content: "var key = \"AKIAIOSFODNN7EXAMPLE\"",
			expectLines: map[string]int{
				"AWS Access Key": 1,
			},
		},
		{
			name:    "Second line match",
			content: "safe\nvar key = \"AKIAIOSFODNN7EXAMPLE\"",
			expectLines: map[string]int{
				"AWS Access Key": 2,
			},
		},
		{
			name:    "Third line match with blank lines",
			content: "\n\nvar key = \"AKIAIOSFODNN7EXAMPLE\"",
			expectLines: map[string]int{
				"AWS Access Key": 3,
			},
		},
		{
			name:    "Match at start of line",
			content: "AKIAIOSFODNN7EXAMPLE",
			expectLines: map[string]int{
				"AWS Access Key": 1,
			},
		},
		{
			name:    "Match after newline",
			content: "\nAKIAIOSFODNN7EXAMPLE",
			expectLines: map[string]int{
				"AWS Access Key": 2,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			findings, err := scanner.Scan(tt.content)
			if err != nil {
				t.Fatalf("Scan failed: %v", err)
			}

			if len(findings) == 0 {
				t.Fatalf("Expected findings, got none")
			}

			for _, f := range findings {
				expectedLine, ok := tt.expectLines[f.Type]
				if ok {
					if f.Line != expectedLine {
						t.Errorf("Finding %s: expected line %d, got %d", f.Type, expectedLine, f.Line)
					}
				}
			}
		})
	}
}
