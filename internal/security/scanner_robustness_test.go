package security

import (
	"testing"
)

func TestScanner_Robustness(t *testing.T) {
	scanner := NewRegexScanner()

	tests := []struct {
		name          string
		content       string
		shouldBlock   bool
		expectedType  string
	}{
		// Allowed cases (previously false positives)
		{
			name:        "rm -rf *",
			content:     "rm -rf *",
			shouldBlock: false,
		},
		{
			name:        "cat my.config",
			content:     "cat my.config",
			shouldBlock: false,
		},
		{
			name:        "cat config.json",
			content:     "cat config.json",
			shouldBlock: false,
		},
		{
			name:        "rm -rf tmp/*",
			content:     "rm -rf tmp/*",
			shouldBlock: false,
		},
		// Blocked cases (True Positives)
		{
			name:         "rm -rf /",
			content:      "rm -rf /",
			shouldBlock:  true,
			expectedType: "Root Deletion",
		},
		{
			name:         "rm -rf /*",
			content:      "rm -rf /*",
			shouldBlock:  true,
			expectedType: "Root Deletion",
		},
		{
			name:         "rm -rf ~",
			content:      "rm -rf ~",
			shouldBlock:  true,
			expectedType: "Root Deletion",
		},
		{
			name:         "cat .config",
			content:      "cat .config",
			shouldBlock:  true,
			expectedType: "Dangerous Command",
		},
		{
			name:         "cat ~/.config",
			content:      "cat ~/.config",
			shouldBlock:  true,
			expectedType: "Dangerous Command",
		},
		{
			name:         "cat .ssh/id_rsa",
			content:      "cat .ssh/id_rsa",
			shouldBlock:  true,
			expectedType: "Dangerous Command",
		},
		{
			name:         "cat \"my.config\"", // quoted but valid file, but regex might be tricky
			content:      "cat \"my.config\"",
			shouldBlock:  false, // Should be allowed
		},
		{
			name:         "cat \".config\"", // quoted sensitive file
			content:      "cat \".config\"",
			shouldBlock:  true,
			expectedType: "Dangerous Command",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			findings, err := scanner.Scan(tt.content)
			if err != nil {
				t.Fatalf("Scan failed: %v", err)
			}

			if tt.shouldBlock {
				if len(findings) == 0 {
					t.Errorf("Expected %s to be blocked, but it was allowed", tt.content)
				} else if findings[0].Type != tt.expectedType {
					t.Errorf("Expected blocking type %s, got %s", tt.expectedType, findings[0].Type)
				}
			} else {
				if len(findings) > 0 {
					t.Errorf("Expected %s to be allowed, but it was blocked by %s", tt.content, findings[0].Type)
				}
			}
		})
	}
}
