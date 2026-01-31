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
		// New Regression Tests
		{
			name:        "Safe File Config",
			content:     "cat my.config.json",
			wantFinding: "",
		},
		{
			name:        "Safe File Webpack",
			content:     "cat webpack.config.js",
			wantFinding: "",
		},
		{
			name:        "Dangerous Config Access",
			content:     "cat .config/foo",
			wantFinding: "Dangerous Command",
		},
		{
			name:        "Dangerous Config Access Quoted",
			content:     "cat \".config\"",
			wantFinding: "Dangerous Command",
		},
		{
			name:        "Dangerous Config Access Single Quoted",
			content:     "cat '.config'",
			wantFinding: "Dangerous Command",
		},
		{
			name:        "Dangerous Config Access Home",
			content:     "cat ~/.config/foo",
			wantFinding: "Dangerous Command",
		},
		{
			name:        "Dangerous SSH Access",
			content:     "cat .ssh/id_rsa",
			wantFinding: "Dangerous Command",
		},
		{
			name:        "Dangerous Etc Passwd",
			content:     "cat /etc/passwd",
			wantFinding: "Dangerous Command",
		},
		{
			name:        "Dangerous Etc Passwd Relative",
			content:     "cat etc/passwd",
			wantFinding: "Dangerous Command",
		},
		{
			name:        "Dangerous Etc Passwd Quoted",
			content:     "cat \"etc/passwd\"",
			wantFinding: "Dangerous Command",
		},
		{
			name:        "Safe Etc Similar",
			content:     "cat fooetc/passwd",
			wantFinding: "",
		},
		{
			name:        "Commented Dangerous Command",
			content:     "# rm -rf /",
			wantFinding: "",
		},
		{
			name:        "Indented Commented Dangerous Command",
			content:     "  # rm -rf /",
			wantFinding: "",
		},
		{
			name:        "Real Dangerous Command",
			content:     "rm -rf /",
			wantFinding: "Root Deletion",
		},
		{
			name:        "Real Dangerous Command Multiline",
			content:     "echo hi\nrm -rf /",
			wantFinding: "Root Deletion",
		},
		{
			name:        "Root Deletion Bypass",
			content:     "rm -rf / # comment",
			wantFinding: "Root Deletion",
		},
		{
			name:        "Root Deletion Separator",
			content:     "rm -rf /; echo bye",
			wantFinding: "Root Deletion",
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
