package security

import (
	"testing"
)

func TestRegexScanner_Scan(t *testing.T) {
	scanner := NewRegexScanner()

	tests := []struct {
		name        string
		content     string
		wantFinding string
	}{
		{
			name:        "Safe Content",
			content:     "fmt.Println(\"Hello World\")",
			wantFinding: "",
		},
		{
			name:        "AWS Key",
			content:     "var key = \"AKIAIOSFODNN7EXAMPLE\"",
			wantFinding: "AWS Access Key",
		},
		{
			name:        "GitHub Token",
			content:     "token = \"ghp_123456789012345678901234567890123456\"",
			wantFinding: "GitHub Token",
		},
		{
			name:        "Private Key",
			content:     "-----BEGIN RSA PRIVATE KEY-----\nMIIEpQIBAAKCAQEA...",
			wantFinding: "Private Key",
		},
		{
			name:        "Generic API Key",
			content:     "api_key = \"abc1234567890abc1234567890\"",
			wantFinding: "Generic API Token",
		},
		// Path Traversal Tests
		{
			name:        "Path Traversal - rm",
			content:     "rm -rf ../secret.txt",
			wantFinding: "Path Traversal",
		},
		{
			name:        "Path Traversal - cp",
			content:     "cp ../source .",
			wantFinding: "Path Traversal",
		},
		{
			name:        "Path Traversal - chained command",
			content:     "cd /tmp; rm ../file",
			wantFinding: "Path Traversal",
		},
		{
			name:        "Path Traversal - piped command",
			content:     "echo 'y' | rm ../file",
			wantFinding: "Path Traversal",
		},
		{
			name:        "Path Traversal - sudo",
			content:     "sudo rm ../file",
			wantFinding: "Path Traversal",
		},
		{
			name:        "Path Traversal - subshell",
			content:     "(rm ../file)",
			wantFinding: "Path Traversal",
		},
		{
			name:        "False Positive - echo rm",
			content:     "echo \"rm ../file\"",
			wantFinding: "", // Should NOT find Path Traversal
		},
		{
			name:        "False Positive - echo with quote",
			content:     "echo \"don't run rm ../file\"",
			wantFinding: "",
		},
		{
			name:        "False Positive - comment",
			content:     "# This script uses rm ../file",
			wantFinding: "", // Ideally shouldn't match, but regex might not handle comments yet.
			// My regex `(?:^|...)` matches start of line `rm`.
			// If `# ` precedes `rm`, it's not in separator list.
			// `#` is NOT in separator list. So `# rm` should be safe.
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
					// Filter for relevant finding if we care about specific ones, but here we expect NONE for the tested type
					// However, if we trigger *other* findings, that's fine? No, we want clean output for safe content.
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
