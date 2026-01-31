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
		{
			name:        "Root Deletion (/) - Blocked",
			content:     "rm -rf /",
			wantFinding: "Root Deletion",
		},
		{
			name:        "Root Deletion (/*) - Blocked",
			content:     "rm -rf /*",
			wantFinding: "Root Deletion",
		},
		{
			name:        "Home Deletion (~) - Blocked",
			content:     "rm -rf ~",
			wantFinding: "Root Deletion",
		},
		{
			name:        "Current Dir Wildcard (*) - Allowed",
			content:     "rm -rf *",
			wantFinding: "",
		},
		{
			name:        "Safe Config File (my.config.json) - Allowed",
			content:     "rm my.config.json",
			wantFinding: "",
		},
		{
			name:        "Sensitive Config (.config) - Blocked",
			content:     "rm -rf .config",
			wantFinding: "Dangerous Command",
		},
		{
			name:        "Sensitive File in Home (~/.ssh/id_rsa) - Blocked",
			content:     "cat ~/.ssh/id_rsa",
			wantFinding: "Dangerous Command",
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
