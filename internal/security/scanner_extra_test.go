package security

import (
	"testing"
)

func TestRegexScanner_ExtraScan(t *testing.T) {
	scanner := NewRegexScanner()

	tests := []struct {
		name        string
		content     string
		wantFinding string
	}{
		{
			name:        "Google API Key",
			content:     "const googleKey = \"AIzaSyD-1234567890abcdef1234567890abcde\"",
			wantFinding: "Google API Key",
		},
		{
			name:        "Google API Key In Config",
			content:     "GEMINI_API_KEY=AIzaSyD-1234567890abcdef1234567890abcde",
			wantFinding: "Google API Key",
		},
		{
			name:        "System Deletion /etc",
			content:     "rm -rf /etc",
			wantFinding: "System Deletion",
		},
		{
			name:        "System Deletion /boot",
			content:     "rm -rf /boot",
			wantFinding: "System Deletion",
		},
		{
			name:        "System Deletion /var/log",
			content:     "rm -rf /var/log",
			wantFinding: "System Deletion",
		},
		{
			name:        "System Deletion /usr",
			content:     "rm -rf /usr",
			wantFinding: "System Deletion",
		},
		{
			name:        "Safe Deletion Local /etc",
			content:     "rm -rf ./etc",
			wantFinding: "",
		},
		{
			name:        "Safe Deletion Local /var",
			content:     "rm -rf ./var",
			wantFinding: "",
		},
		{
			name:        "Safe Deletion Relative",
			content:     "rm -rf some/path",
			wantFinding: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			findings, err := scanner.Scan(tt.content)
			if err != nil {
				t.Fatalf("Scan failed: %v", err)
			}

			if tt.wantFinding == "" {
				if len(findings) > 0 {
					t.Errorf("Expected no findings, got %d: %v", len(findings), findings)
				}
			} else {
				if len(findings) == 0 {
					t.Errorf("Expected finding %q, got none", tt.wantFinding)
				} else {
					found := false
					for _, f := range findings {
						if f.Type == tt.wantFinding {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("Expected finding type %q, got %v", tt.wantFinding, findings)
					}
				}
			}
		})
	}
}
