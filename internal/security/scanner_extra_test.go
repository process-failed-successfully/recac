package security

import (
	"strings"
	"testing"
)

func TestRegexScanner_LineByLine(t *testing.T) {
	scanner := NewRegexScanner()

	tests := []struct {
		name          string
		content       string
		shouldBlock   bool
		blockedReason string // partial match of description
	}{
		{
			name:        "Root Deletion at end",
			content:     "rm -rf /",
			shouldBlock: true,
			blockedReason: "Root Deletion",
		},
		{
			name:        "Root Deletion in middle",
			content:     "rm -rf /\necho done",
			shouldBlock: true,
			blockedReason: "Root Deletion",
		},
		{
			name:        "Safe command",
			content:     "ls -la",
			shouldBlock: false,
		},
		{
			name:        "Commented Root Deletion",
			content:     "# rm -rf /",
			shouldBlock: false, // Should NOT block
		},
		{
			name:        "Commented Root Deletion with text",
			content:     "# Be careful not to run rm -rf / here",
			shouldBlock: false, // Should NOT block
		},
        {
            name:        "Star Deletion",
            content:     "rm -rf *",
            shouldBlock: false, // Allowed for workspace cleanup
        },
        {
            name:        "Star Deletion in middle",
            content:     "rm -rf *\necho cleaning done",
            shouldBlock: false, // Allowed for workspace cleanup
        },
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			findings, err := scanner.Scan(tc.content)
			if err != nil {
				t.Fatalf("Scan failed: %v", err)
			}

			if tc.shouldBlock {
				if len(findings) == 0 {
					t.Errorf("Expected findings for '%s', got none", tc.content)
				} else {
                    found := false
                    for _, f := range findings {
                        if strings.Contains(f.Description, tc.blockedReason) {
                            found = true
                            break
                        }
                    }
                    if !found {
                        t.Errorf("Expected finding '%s', got %v", tc.blockedReason, findings)
                    }
                }
			} else {
				if len(findings) > 0 {
					t.Errorf("Expected no findings for '%s', got %v", tc.content, findings)
				}
			}
		})
	}
}
