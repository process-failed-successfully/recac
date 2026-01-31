package security

import (
	"testing"
)

func TestScanner_Reproduction_FalsePositives(t *testing.T) {
	scanner := NewRegexScanner()

	tests := []struct {
		name    string
		content string
		want    int // Number of findings
	}{
		{
			name:    "Inline comment with dangerous command",
			content: "echo 'hello' # rm -rf /",
			want:    0, // Should be ignored
		},
		{
			name:    "Command inside string",
			content: `echo "Don't run rm -rf /"`,
			want:    0, // Should be ignored
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			findings, err := scanner.Scan(tt.content)
			if err != nil {
				t.Fatalf("Scan failed: %v", err)
			}
			if len(findings) != tt.want {
				t.Errorf("got %d findings, want %d", len(findings), tt.want)
				for _, f := range findings {
					t.Logf("Finding: %s: %s", f.Type, f.Description)
				}
			}
		})
	}
}
